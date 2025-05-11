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
	PQIExcellent         float64 `json:"poiExcellent"`
	BridgeRate           float64 `json:"bridgeRate"`
	RecycleRate          float64 `json:"recycleRate"`
	NationalMQIEast      float64 `json:"nationalMqiEast"`
	NationalMQICentral   float64 `json:"nationalMqiCentral"`
	NationalMQIWest      float64 `json:"nationalMqiWest"`
	NationalPQIEast      float64 `json:"nationalPqiEast"`
	NationalPQICentral   float64 `json:"nationalPqiCentral"`
	NationalPQIWest      float64 `json:"nationalPqiWest"`
	ProvincialMQIEast    float64 `json:"provincialMqiEast"`
	ProvincialMQICentral float64 `json:"provincialMqiCentral"`
	ProvincialMQIWest    float64 `json:"provincialMqiWest"`
	ProvincialPQIEast    float64 `json:"provincialPqiEast"`
	ProvincialPQICentral float64 `json:"provincialPqiCentral"`
	ProvincialPQIWest    float64 `json:"provincialPqiWest"`
	RuralMQI             float64 `json:"ruralMqi"`
	MaintenanceRate      float64 `json:"maintenanceRate"`
}
