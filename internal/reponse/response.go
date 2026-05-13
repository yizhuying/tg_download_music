package reponse

import "github.com/gin-gonic/gin"

func Success(c *gin.Context, data interface{}) {
	c.JSON(200, gin.H{
		"ok":      true,
		"data":    data,
		"message": "操作成功",
	})
}

func Error(c *gin.Context, message string) {
	c.JSON(200, gin.H{
		"ok":      false,
		"message": message,
	})
}
