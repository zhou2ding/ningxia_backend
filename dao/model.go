package dao

import "gorm.io/gorm"

type Road struct {
	gorm.Model `json:"-"`
	Name       string `json:"name" gorm:"unique"`
}

type ProvinceSetting struct {
	gorm.Model        `json:"-"`
	Year              int     `json:"year" gorm:"unique"`
	Expressway        float64 `json:"expressway"`
	NationalHighway   float64 `json:"nationalHighway"`
	ProvincialHighway float64 `json:"provincialHighway"`
	RuralRoad         float64 `json:"ruralRoad"`
}

type NationalSetting struct {
	gorm.Model           `json:"-"`
	Plan                 string  `json:"plan" gorm:"unique"`
	MQIExcellent         float64 `json:"mqiExcellent"`
	POIExcellent         float64 `json:"poiExcellent"`
	BridgeRate           float64 `json:"bridgeRate"`
	RecycleRate          float64 `json:"recycleRate"`
	NationalMQIEast      float64 `json:"nationalMqiEast"`
	NationalMQICentral   float64 `json:"nationalMqiCentral"`
	NationalMQIWest      float64 `json:"nationalMqiWest"`
	NationalPOIEast      float64 `json:"nationalPoiEast"`
	NationalPOICentral   float64 `json:"nationalPoiCentral"`
	NationalPOIWest      float64 `json:"nationalPoiWest"`
	ProvincialMQIEast    float64 `json:"provincialMqiEast"`
	ProvincialMQICentral float64 `json:"provincialMqiCentral"`
	ProvincialMQIWest    float64 `json:"provincialMqiWest"`
	ProvincialPOIEast    float64 `json:"provincialPoiEast"`
	ProvincialPOICentral float64 `json:"provincialPoiCentral"`
	ProvincialPOIWest    float64 `json:"provincialPoiWest"`
	RuralMQI             float64 `json:"ruralMqi"`
	MaintenanceRate      float64 `json:"maintenanceRate"`
}
