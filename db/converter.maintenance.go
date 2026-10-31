package db

import (
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db/generated"
)

func GameMaintenanceToMaintenance(source cs.DesignMaintenance) *generated.DesignMaintenance {
	value := generated.DesignMaintenance(source)
	return &value
}

func MaintenanceToGameMaintenance(source *generated.DesignMaintenance) cs.DesignMaintenance {
	if source == nil {
		return nil
	}
	return cs.DesignMaintenance(*source)
}

func GameAutomationToAutomation(source cs.LogisticsAutomation) *generated.LogisticsAutomation {
	value := generated.LogisticsAutomation(source)
	return &value
}
func AutomationToGameAutomation(source *generated.LogisticsAutomation) cs.LogisticsAutomation {
	if source == nil {
		return cs.LogisticsAutomation{}
	}
	return cs.LogisticsAutomation(*source)
}
