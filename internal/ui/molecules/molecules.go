package molecules

import (
	"github.com/charmbracelet/bubbles/list"
)

type MenuItem struct {
	TitleStr, DescStr string
	ActionStr         string
}

func (i MenuItem) Title() string       { return i.TitleStr }
func (i MenuItem) Description() string { return i.DescStr }
func (i MenuItem) FilterValue() string { return i.TitleStr }

func NewMenuItem(title, desc, action string) list.Item {
	return MenuItem{TitleStr: title, DescStr: desc, ActionStr: action}
}
