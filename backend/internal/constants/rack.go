package constants

type RackStatus string

const (
	RackAvailable   RackStatus = "available"
	RackReserved    RackStatus = "reserved"
	RackUnavailable RackStatus = "unavailable"
	RackMaintenance RackStatus = "maintenance"
)

func ValidRackStatus(value RackStatus) bool {
	switch value {
	case RackAvailable, RackReserved, RackUnavailable, RackMaintenance:
		return true
	default:
		return false
	}
}

func RackStatusValues() []RackStatus {
	return []RackStatus{RackAvailable, RackReserved, RackUnavailable, RackMaintenance}
}

func RackCanReceiveLoad(value RackStatus) bool {
	return value == RackAvailable || value == RackReserved
}
