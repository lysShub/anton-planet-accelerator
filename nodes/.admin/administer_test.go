package admin_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestXxxx(t *testing.T) {

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	err := r.Run(":19986")
	fmt.Println(err)
}
