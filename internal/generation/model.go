package generation

type Status string

const (
	Pending Status = "pending"
	Done    Status = "done"
	Failed  Status = "failed"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
