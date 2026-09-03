package tools

func init() {
	RegisterTool(NewScanTool())
	RegisterTool(NewCheckMigrationTool())
	RegisterTool(NewExplainRuleTool())
}
