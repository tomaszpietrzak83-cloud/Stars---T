package generated

import (
	"database/sql/driver"
	"github.com/sirgwain/craig-stars/cs"
)

type DesignMaintenance cs.DesignMaintenance
type LogisticsAutomation cs.LogisticsAutomation

func (item *LogisticsAutomation) Value() (driver.Value, error) { return valueJSON(item) }
func (item *LogisticsAutomation) Scan(src interface{}) error   { return scanJSON(src, item) }

func (item *DesignMaintenance) Value() (driver.Value, error) { return valueJSON(item) }
func (item *DesignMaintenance) Scan(src interface{}) error   { return scanJSON(src, item) }
