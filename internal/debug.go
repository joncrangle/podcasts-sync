package internal

type Debug struct {
	DTitle       string
	DDescription string
}

func (d Debug) Title() string { return d.DTitle }

func (d Debug) Description() string { return d.DDescription }

func (d Debug) FilterValue() string { return "" }
