package main

import (
	"context"
	"sync"

	"github.com/gin-gonic/gin"
)

type Ddns struct {
	sync.Mutex
	dns map[string]string
}

func NewDdns() *Ddns {
	return &Ddns{
		dns: make(map[string]string),
	}
}

func (d *Ddns) Run(ctx context.Context) error {
	e := gin.Default()
	e.GET("/dns", func(c *gin.Context) {
		d.Lock()
		defer d.Unlock()
		name := c.Query("name")
		addr := c.Query("addr")
		d.dns[name] = addr
		c.String(200, "OK")
	})
	d.Lock()
	defer d.Unlock()
	e.GET("/dnss", func(c *gin.Context) {
		c.JSON(200, d.dns)
	})
	return e.Run(":8080")
}
