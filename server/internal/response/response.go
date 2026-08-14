package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: 0, Data: data})
}

func OKMsg(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: msg})
}

func OKMsgData(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: msg, Data: data})
}

func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Response{Code: http.StatusBadRequest, Message: msg})
}

func ServerError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, Response{Code: http.StatusInternalServerError, Message: msg})
}
