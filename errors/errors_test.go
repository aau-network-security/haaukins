package errors

import (
	"github.com/sirupsen/logrus"
	"testing"
)

func TestE(t *testing.T) {

	tests := []struct {
		functionCall FCall
		message   string
		level logrus.Level
	}{
	  {functionCall:"daemon.New", message:"error on daemon.New function",level:logrus.ErrorLevel},
	  {functionCall:"daemon.New", message:"Daemon has been created",level:logrus.InfoLevel},
	  {functionCall:"auth.NewAuthenticator",message:"key is plain text",level:logrus.WarnLevel},
	  {functionCall:"eventpool.RemoveEvent",message: "Removing event.",level:logrus.DebugLevel},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			if err := E(tt.functionCall, tt.message); err.Error() != tt.message {
				t.Errorf("E() error = %v", err)
			}
		})
	}
}

func TestSeverity(t *testing.T){
	const functioncall FCall =  "TestSeverity"
	message := "erorr to test out severity of logrus levels"
	tests:=[] struct{
		functionCall FCall
		message string
		level logrus.Level
	}{
		{functionCall:functioncall,message:message,level:logrus.ErrorLevel},
		{functionCall:functioncall,message:message,level:logrus.DebugLevel},
		{functionCall:functioncall,message:message,level:logrus.WarnLevel},
		{functionCall:functioncall,message:message,level:logrus.InfoLevel},
		{functionCall:functioncall,message:message,level:logrus.TraceLevel},
	}
	for _, tt  :=range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			if err := E(tt.functionCall,tt.message,tt.level); Severity(err) != tt.level {
				t.Fatalf("Error on getting severity %v",err)
			}
		})

	}
}

