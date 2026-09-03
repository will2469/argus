---
name: argus-mcp
description: "Authoritative architectural guidance and operational engineering harness for Model Context Protocol (MCP) version 2026-07-28 and dual-track implementation in Argus. Auto-triggers when authoring, reviewing, debugging, or upgrading MCP servers, implementing stateless protocol handlers, server/discover RPC, Multi Round-Trip Requests (MRTR / SEP-2322), _meta response caching, or evaluating protocol deprecations ('mcp 2026-07-28', 'stateless mcp', 'server/discover', 'SEP-2322', 'MRTR', 'argus-mcp', 'mcp upgrade', '_meta protocolVersion', 'dual-track mcp')."
compatibility: "Requires modern agentic IDE environment, Go 1.25+, bash, and git"
metadata:
  version: "1.0.0"
  author: "Will (https://github.com/will2469)"
  license: "CC-BY-NC-4.0"
  citations:
    - "Model Context Protocol Specification (Revision 2026-07-28): https://modelcontextprotocol.io/specification/2026-07-28"
    - "SEP-2322: Multi Round-Trip Requests (MRTR) for Stateless Execution: https://modelcontextprotocol.io/proposals/sep-2322"
    - "JSON-RPC 2.0 Specification (2010): https://www.jsonrpc.org/specification"
    - "RFC 8259: The JavaScript Object Notation (JSON) Data Interchange Format (IETF)"
    - "ArXiv 2607.25032: Authoring Agent Skills: A Software-Engineering Approach"
---

# Argus MCP Engineering Skill (`2026-07-28` Standard)

> **Core Thesis:** Pembaruan Model Context Protocol (MCP) revisi **2026-07-28** menandai pergeseran paradigma paling fundamental dalam sejarah protokol: **penghapusan total konsep sesi stateful** demi arsitektur **stateless request/response**. Setiap request wajib bersifat *self-describing* membawa blok `_meta`, dan server dilarang mengandalkan request sebelumnya untuk mengetahui identitas maupun kapabilitas klien. Untuk menjamin stabilitas produksi, sistem wajib menerapkan arsitektur **Dual-Track** tanpa *breaking change* bagi klien legacy.

---

## 1. Arsitektur Dual-Track MCP (`2026-07-28` vs Legacy)

Protokol membagi eksekusi di titik paling awal (*single boundary gate* pada `ValidateJSONRPC`):

```
                                  [Incoming Request]
                                          │
                                          ▼
                            ┌───────────────────────────┐
                            │   ValidateJSONRPC Gate    │
                            │ (Extracts _meta if pres.) │
                            └─────────────┬─────────────┘
                                          │
                   ┌──────────────────────┴──────────────────────┐
                   │                                             │
                   ▼ (Has _meta.protocolVersion)                 ▼ (No _meta / Legacy)
       ┌───────────────────────────────┐             ┌───────────────────────────────┐
       │   STATELESS PATH (2026-07-28) │             │    LEGACY PATH (2024-11-05)   │
       │───────────────────────────────│             │───────────────────────────────│
       │ • Per-request validation      │             │ • 3-state lifecycle machine   │
       │ • No mutex / session lock     │             │ • PreInit -> Init -> Ready    │
       │ • Server/discover idempotent  │             │ • initialize handshake req.   │
       │ • Zero prior-request memory   │             │ • sess.protocolVersion cache  │
       └───────────────┬───────────────┘             └───────────────┬───────────────┘
                       │                                             │
                       └──────────────────────┬──────────────────────┘
                                              │
                                              ▼
                                    ┌───────────────────┐
                                    │ Tool Concurrency  │
                                    │ & Dispatch Pool   │
                                    └───────────────────┘
```

---

## 2. Matriks Deprecasi Spesifikasi `2026-07-28`

Berdasarkan spesifikasi resmi `2026-07-28`, fitur-fitur era `2024-11-05` s/d `2025-11-25` berikut dinyatakan **DEPRECATED**:

| Komponen Protokol | Status di `2026-07-28` | Alasan & Dampak Teknis | Standar Pengganti Modern |
| :--- | :--- | :--- | :--- |
| **Stateful Lifecycle** | **DEPRECATED** | Menghambat horizontal scaling dan load balancer round-robin. | **Stateless Per-Request** (`_meta`) & query RPC `server/discover`. |
| **Session Tracking** (`Mcp-Session-Id`) | **DEPRECATED** | Mengharuskan sticky connection yang rapuh pada cloud-native. | **Self-describing requests** (transport-agnostic). |
| **Protocol Roots** (`roots/list`) | **DEPRECATED** | Tidak aman mengandalkan batas direktori dari sisi klien. | **Server-side containment** deterministik via `PathAuthority`. |
| **Sampling** (`sampling/createMessage`) | **DEPRECATED** | Memerlukan koneksi dua arah persisten yang kompleks. | **MRTR (SEP-2322)**: pola kelanjutan berbasis `requestState`. |
| **Protocol Logging** (`notifications/message`) | **DEPRECATED** | Menghasilkan noise pada kanal data JSON-RPC. | **Observability native** (OpenTelemetry, structured stderr/slog). |
| **Un-cached Listings** | **DEPRECATED** | Redundan dan memboroskan token/latensi. | **Metadata Caching** (`_meta: {ttlMs, cacheScope}`). |

---

## 3. Pola Multi Round-Trip Requests (MRTR / SEP-2322)

Untuk skenario interaktif (Human-in-the-Loop, konfirmasi destruktif, input tambahan), spec `2026-07-28` menggantikan *sampling/elicitation* berbasis sesi dengan MRTR:

```
Client                                                  Server (Instance A / B)
  │                                                               │
  │─── 1. tools/call (payload awal, _meta) ──────────────────────►│
  │                                                               │ (Needs user confirmation)
  │◄── 2. Result { resultType: "input_required", requestState } ──│
  │                                                               │
  │ [Client prompts user for confirmation / approval token]       │
  │                                                               │
  │─── 3. tools/call (payload, inputResponses, requestState) ────►│ (Can hit Instance B!)
  │                                                               │ (Resumes statelessly via token)
  │◄── 4. Result { resultType: "complete", content: [...] } ──────│
```

### Invariant Teknis MRTR:
1. `resultType: "input_required"`: Mengindikasikan eksekusi tertunda menunggu input lanjutan.
2. `requestState`: Token kriptografis/terenkripsi buram (*opaque blob*) yang menyimpan status eksekusi.
3. Klien memanggil ulang `tools/call` dengan menyertakan `requestState` dan `inputResponses`. Instance server mana pun dapat melanjutkan eksekusi tanpa ketergantungan memori sesi lokal.

---

## 4. Guardrail Implementasi Argus MCP

Saat mengembangkan atau memodifikasi modul `shared/mcp/`, patuhi aturan ketat berikut:

### 1. Zero Session Leakage
- Struct `serverSession` adalah **murni untuk jalur legacy** (`2024-11-05` s/d `2025-11-25`).
- Request stateless (`req.Meta != nil && isStatelessEra(req.Meta.ProtocolVersion)`) **DILARANG KERAS** membaca atau menulis field apa pun pada `serverSession`.

### 2. Thread-Safe Registry (`sync.RWMutex`)
- `tools.Registry` wajib dilindungi oleh `sync.RWMutex` untuk mencegah data race saat dynamic registration, unregistering, dan concurrent dispatching.
- Pada method `Dispatch()`, `RLock()` hanya dipegang saat *lookup* tool dan segera dilepas sebelum eksekusi pipeline validasi atau runner dimulai.

### 3. Anti-Fat Boundary (~250 Baris/File)
- Setiap file Go di `shared/mcp/` dibatasi maksimal **~250 baris**.
- Jika komponen bertambah, lakukan dekomposisi kohesif:
  - `server.go`: Loop pembacaan framing stdio dan routing request.
  - `session.go`: State machine legacy dan penanganan notifikasi.
  - `transport/types.go`: Struct data transport (`RequestMeta`, `ParsedRequest`).
  - `transport/validator.go`: Gerbang tunggal RFC JSON-RPC 2.0.

### 4. Per-Request Error Handling
- Kegagalan versi protokol pada jalur stateless (`!SupportedProtocolVersions[req.Meta.ProtocolVersion]`) dibalas dengan error JSON-RPC per-request (`CodeInvalidParams` `-32602` / `CodeUnsupportedProtocolVersion` `-32022`).
- **JANGAN PERNAH** memutus koneksi stdio akibat kesalahan versi per-request; request berikutnya dalam stream yang sama tetap independen.

---

## 5. Protokol Verifikasi & Gauntlet Testing

Setiap perubahan pada layer MCP wajib melalui pengujian bertingkat:

```bash
# 1. Race detector wajib dijalankan pada seluruh paket MCP
go test -race -v ./shared/mcp/...

# 2. Gauntlet adversarial testing (cancellation, framing, boundary rejections)
go test -v -run "TestGauntlet_|TestTransport_|TestServe_" ./shared/mcp/tests/...

# 3. Stateless vs Legacy parity assertion
go test -v -run "TestServerDiscover_|TestToolsList_Stateless" ./shared/mcp/tests/...

# 4. Global workspace regression check
go test ./...
go vet ./...
```

---

## 6. Checklist Kualitas Skill Argus MCP

Gunakan checklist ini sebelum menyelesaikan tugas terkait integrasi MCP:
- [ ] **Dual-Track Active**: Jalur stateless (`2026-07-28`) dan legacy (`2024-11-05`) berjalan independen.
- [ ] **Discoverable**: RPC `server/discover` mengembalikan daftar versi lengkap dan idempoten.
- [ ] **Cache Hints**: Endpoint listing (`tools/list`) mengembalikan metadata TTL dan cacheScope.
- [ ] **Thread Safe**: Mutex terpasang pada seluruh akses stateful registry.
- [ ] **Zero Race**: `go test -race ./shared/mcp/...` menghasilkan status PASS (0 data race).

---

## 7. Rujukan Otoritatif & Bacaan Lanjutan

- Detail lengkap skema request/response dan spesifikasi teknis: [spec_2026_07_28.md](file:///home/will/Monorepo/argus/.agents/skills/argus-mcp/references/spec_2026_07_28.md).
- Spesifikasi Resmi Model Context Protocol: [modelcontextprotocol.io/specification/2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28).
- Proposal Peningkatan SEP-2322 (MRTR): [modelcontextprotocol.io/proposals/sep-2322](https://modelcontextprotocol.io/proposals/sep-2322).
