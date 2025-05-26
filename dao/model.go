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
	gorm.Model `json:"-"`
	RoadType   string `json:"roadType" gorm:"unique"` // 保持不变，用于数据库查询

	// 高速公路 Expressway fields
	ExpresswayPqiTarget     float64 `json:"expressway_pqiTarget"`
	ExpresswayNetworkPQI    float64 `json:"expressway_networkPQI"`
	ExpresswayExcellentRate float64 `json:"expressway_excellentRate"`
	ExpresswayUnitPQI       float64 `json:"expressway_unitPQI"`
	ExpresswayUnitPCI       float64 `json:"expressway_unitPCI"`
	ExpresswayUnitRQI       float64 `json:"expressway_unitRQI"`
	ExpresswayUnitRDI       float64 `json:"expressway_unitRDI"`
	ExpresswayUnitSRI       float64 `json:"expressway_unitSRI"`

	// 国省干线 National/Provincial fields
	NationalProvincialPqiTarget             float64 `json:"nationalProvincial_pqiTarget"`
	NationalProvincialNetworkPQI1           float64 `json:"nationalProvincial_networkPQI1"`
	NationalProvincialNetworkExcellentRate1 float64 `json:"nationalProvincial_networkExcellentRate1"`
	NationalProvincialNetworkPQI2           float64 `json:"nationalProvincial_networkPQI2"`
	NationalProvincialNetworkExcellentRate2 float64 `json:"nationalProvincial_networkExcellentRate2"`
	NationalProvincialUnitPQI1              float64 `json:"nationalProvincial_unitPQI1"`
	NationalProvincialUnitPCI1              float64 `json:"nationalProvincial_unitPCI1"`
	NationalProvincialUnitRQI1              float64 `json:"nationalProvincial_unitRQI1"`
	NationalProvincialUnitRDI1              float64 `json:"nationalProvincial_unitRDI1"`
	NationalProvincialUnitSRI1              float64 `json:"nationalProvincial_unitSRI1"`
	NationalProvincialUnitPQI2              float64 `json:"nationalProvincial_unitPQI2"`
	NationalProvincialUnitPCI2              float64 `json:"nationalProvincial_unitPCI2"`
	NationalProvincialUnitRQI2              float64 `json:"nationalProvincial_unitRQI2"`

	// 农村公路 Rural fields
	RuralPqiTarget   float64 `json:"rural_pqiTarget"`
	RuralNetworkPQI1 float64 `json:"rural_networkPQI1"`
	RuralNetworkPQI2 float64 `json:"rural_networkPQI2"`
	RuralUnitPQI1    float64 `json:"rural_unitPQI1"`
	RuralUnitPCI1    float64 `json:"rural_unitPCI1"`
	RuralUnitRQI1    float64 `json:"rural_unitRQI1"`
	RuralUnitRDI1    float64 `json:"rural_unitRDI1"`
	RuralUnitSRI1    float64 `json:"rural_unitSRI1"`
	RuralUnitPQI2    float64 `json:"rural_unitPQI2"`
	RuralUnitPCI2    float64 `json:"rural_unitPCI2"`
	RuralUnitRQI2    float64 `json:"rural_unitRQI2"`
}
