package model

type SteampipeAdapterStruct struct {
	Table   string
	Convert func(workspaceId string, jsonRow string) (string, bool)
}

func (a *SteampipeAdapterStruct) EntityName() string {
	return a.Table
}

func (a *SteampipeAdapterStruct) ToResourceJson(workspaceId string, sqlRow SqlRow) (string, bool) {
	return a.Convert(workspaceId, sqlRow.Json)
}

type SteampipeAdapter interface {
	EntityName() string
	ToResourceJson(workspaceId string, sqlRow SqlRow) (string, bool)
}
