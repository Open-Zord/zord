package list

// Data agrega os campos de entrada validáveis do use case List (paginação).
type Data struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// Request encapsula Data para o Execute do Service.
type Request struct {
	Data *Data
}

// NewRequest constrói o Request.
func NewRequest(data *Data) *Request {
	return &Request{Data: data}
}

// Validate não rejeita paginação ausente/negativa — o Service aplica defaults.
func (r *Request) Validate() error {
	return nil
}
