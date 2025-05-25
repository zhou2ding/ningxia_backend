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

type CalculationSetting struct {
	gorm.Model    `json:"-"`
	RoadType      string  `json:"roadType" gorm:"unique"`
	PqiTarget     float64 `json:"pqiTarget"`
	NetworkPQI    float64 `json:"networkPQI"`
	NetworkPQI1   float64 `json:"networkPQI1"`
	NetworkPQI2   float64 `json:"networkPQI2"`
	ExcellentRate float64 `json:"excellentRate"`
	UnitPQI       float64 `json:"unitPQI"`
	UnitPCI       float64 `json:"unitPCI"`
	UnitRQI       float64 `json:"unitRQI"`
	UnitRDI       float64 `json:"unitRDI"`
	UnitSRI       float64 `json:"unitSRI"`
	UnitPQI1      float64 `json:"unitPQI1"`
	UnitPCI1      float64 `json:"unitPCI1"`
	UnitRQI1      float64 `json:"unitRQI1"`
	UnitRDI1      float64 `json:"unitRDI1"`
	UnitSRI1      float64 `json:"unitSRI1"`
	UnitPQI2      float64 `json:"unitPQI2"`
	UnitPCI2      float64 `json:"unitPCI2"`
	UnitRQI2      float64 `json:"unitRQI2"`
}
