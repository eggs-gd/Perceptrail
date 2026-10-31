package sample

import "io"

type Reader interface{ Read() }

type Item struct{}

const Version = 1

var cache = map[string]int{}

type state int

func (Item) Read() {}

func New() *Item { return &Item{} }

func (s state) Enqueue() {}

func helper() {}
