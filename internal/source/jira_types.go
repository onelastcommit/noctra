package source

type jiraIssue struct {
	ID     string     `json:"id"`
	Key    string     `json:"key"`
	Fields jiraFields `json:"fields"`
}

type jiraFields struct {
	Summary     string           `json:"summary"`
	Description *jiraADFDocument `json:"description"`
	Status      jiraStatus       `json:"status"`
	Project     jiraProject      `json:"project"`
	Labels      []string         `json:"labels"`
	Comment     jiraCommentPage  `json:"comment"`
}

type jiraStatus struct {
	Name string `json:"name"`
}

type jiraProject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type jiraCommentPage struct {
	Comments []jiraComment `json:"comments"`
}

type jiraComment struct {
	Body   *jiraADFDocument `json:"body"`
	Author jiraUser         `json:"author"`
}

type jiraUser struct {
	DisplayName string `json:"displayName"`
}

type jiraADFDocument struct {
	Type    string           `json:"type"`
	Content []jiraADFContent `json:"content"`
}

type jiraADFContent struct {
	Type    string           `json:"type"`
	Text    string           `json:"text,omitempty"`
	Content []jiraADFContent `json:"content,omitempty"`
}

func (issue jiraIssue) descriptionText() string {
	if issue.Fields.Description == nil {
		return ""
	}
	return adfToText(issue.Fields.Description)
}

func (c jiraComment) bodyText() string {
	if c.Body == nil {
		return ""
	}
	return adfToText(c.Body)
}

type jiraMeta struct {
	BaseURL string
	Key     string
}

type jiraTransition struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	To   jiraStatus `json:"to"`
}
