package route

import "github.com/rhp-QE/roc-foundation-util-go/stringutil"

func UserConnectionsKey(userID string) string {
	return stringutil.FormatKey("im", "user", userID, "conns")
}

func ConnectionKey(connectionID string) string {
	return stringutil.FormatKey("im", "conn", connectionID)
}

func DeviceConnectionKey(userID string, deviceID string) string {
	return stringutil.FormatKey("im", "device", userID, deviceID)
}
