package libs

// Signature base signature struct
type Signature struct {
	ID   string
	Type string
	Info struct {
		Name     string
		Category string
		Risk     string
		Tech     string
		OS       string
	}

	Origin     Request
	Requests   []Request
	RawRequest string
	Payloads   []string
	Params     []map[string]string
	Variables  []map[string]string
	Target     map[string]string
}
