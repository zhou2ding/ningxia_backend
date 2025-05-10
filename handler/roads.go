package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"ningxia_backend/dao"
)

func GetRoads(c *gin.Context) {
	var roads []dao.Road
	if err := dao.GetDB().Find(&roads).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取路线名称失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, roads)
}
