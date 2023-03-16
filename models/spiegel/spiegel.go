package spiegel

type Result struct {
	Url string `json:"url"`
}

type Search struct {
	Results []Result `json:"results"`
}
