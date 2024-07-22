package requests

type Header struct {
	Article		int		`json:"Article"`
	IsReleased	*bool	`json:"IsReleased"`
}
