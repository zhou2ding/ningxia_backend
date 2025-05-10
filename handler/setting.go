package handler

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"ningxia_backend/dao"
	"strconv"
)

func SaveProvinceSettings(c *gin.Context) {
	var setting dao.ProvinceSetting
	if err := c.ShouldBindJSON(&setting); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	dao.GetDB().Save(&setting)
	c.JSON(200, gin.H{"message": "省厅指标保存成功"})
}

func SaveNationalSettings(c *gin.Context) {
	var setting dao.NationalSetting
	if err := c.ShouldBindJSON(&setting); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	dao.GetDB().Save(&setting)
	c.JSON(200, gin.H{"message": "交通部指标保存成功"})
}

func GetProvinceSetting(c *gin.Context) {
	yearStr := c.Param("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的年份参数"})
		return
	}

	var setting dao.ProvinceSetting
	if err = dao.GetDB().Where("year = ?", year).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("未找到%d年的省厅配置", year)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		return
	}

	c.JSON(http.StatusOK, setting)
}

func GetNationalSetting(c *gin.Context) {
	plan := c.Param("plan")

	var setting dao.NationalSetting
	if err := dao.GetDB().Where("plan = ?", plan).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("未找到计划'%s'的交通部配置", plan)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		return
	}

	c.JSON(http.StatusOK, setting)
}
