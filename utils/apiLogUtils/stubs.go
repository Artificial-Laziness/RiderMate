package apiLogUtils

import (
	"log"
)

// LogGeneralInfo logs an informational message with formatting
func LogGeneralInfo(format string, v ...interface{}) {
	log.Printf("[INFO] "+format+"\n", v...)
}

// LogGeneralError logs an error message with formatting
func LogGeneralError(format string, v ...interface{}) {
	log.Printf("[ERROR] "+format+"\n", v...)
}
