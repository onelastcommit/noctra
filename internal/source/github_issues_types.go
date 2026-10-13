package source

type githubIssue struct {
	Number   int             `json:"number"`
	Title    string          `json:"title"`
	Body     string          `json:"body"`
	URL      string          `json:"url"`
	Labels   []githubLabel   `json:"labels"`
	Comments []githubComment `json:"comments"`
}

type githubLabel struct {
	Name string `json:"name"`
}

type githubComment struct {
	Body   string      `json:"body"`
	Author githubActor `json:"author"`
}

type githubActor struct {
	Login string `json:"login"`
}

type githubMeta struct {
	OwnerRepo string
	Number    int
}
