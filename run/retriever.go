package run

type Retriever interface {
	ToTool() AgentTool
}
