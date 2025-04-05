package steampipe

type SteampipeClient struct {
	ExecCmd string `default:"steampipe"`
}


func NewSteampipeClient() *SteampipeClient {
	return &SteampipeClient{}
}


