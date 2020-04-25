package main

import (
	"fmt"
	"github.com/pinke/muses"
	"github.com/pinke/muses/pkg/tpl/tplbeego"
)

var cfg = `
[muses.tmpl.beego]
    debug = true
`

func main() {
	if err := muses.Container(
		[]byte(cfg),
		tplbeego.Register,
	); err != nil {
		panic(err)
	}
	obj, err := tplbeego.Caller()
	if err != nil {
		fmt.Println("err", err)
		return
	}
	obj.Data["hello"] = "hello i2eco"
	output, err := obj.RenderBytes()
	if err != nil {
		fmt.Println("err", err)
		return
	}

	fmt.Println(string(output))
}
