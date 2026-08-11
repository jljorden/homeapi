package greetings

type Greeting struct {
	ID       int64  `json:"id"`
	Greeting string `json:"greeting"`
	Period   string `json:"period"`
}