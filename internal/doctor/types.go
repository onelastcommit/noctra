package doctor

type agentAuth struct {
	method string
	source string
}

type check struct {
	name   string
	ok     bool
	detail string
	hint   string
	note   string
}

type jsonCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Hint   string `json:"hint,omitempty"`
	Note   string `json:"note,omitempty"`
}
