package sample

func helper() {}

type Item struct{}

func New() *Item { return &Item{} }

type Reader interface{ Read() }

var cache = map[string]int{}
