package gui

type ResErr struct {
	Code    int
	Message string
}

type RespondErr struct {
	JsonRPC string
	Id      string
	Error   ResErr
}
