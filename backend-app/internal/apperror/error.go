package apperror

type AppError struct {
	Code    int
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

func New(code int, msg string) error {
	return &AppError{Code: code, Message: msg}
}
