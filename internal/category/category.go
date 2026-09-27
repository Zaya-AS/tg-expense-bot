package category

import "strings"

var Defaults = []string{
	"еда",
	"напитки",
	"продукты",
	"транспорт",
	"дом",
	"здоровье",
	"покупки",
	"развлечения",
	"другое",
}

func Normalize(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
