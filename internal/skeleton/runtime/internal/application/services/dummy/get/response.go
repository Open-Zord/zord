package get

// Response agrega a saída do use case Get.
type Response struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
