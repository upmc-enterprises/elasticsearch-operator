package k8sutil

import (
	"testing"

)


const (
	NonExistingIP = "10.1.1.1"  /* some non existing IP */
	NonExistingPOD = "nonexistingpodname"
)

/*
   Test for Failures, by sending the unrechable ES Master IP, the below function under test  should always return the error, 
   incase if testing function does not return the error then the unit test should fail. 
*/
func Test_scaling_change_setting(t *testing.T) {
	err := ES_change_settings(NonExistingIP, "1m", "request"); 
	if (err == nil){
		t.Errorf("Scaling unittest change setting failed");
	}
}

func Test_check_for_green(t *testing.T) {
	err := ES_checkForGreen(NonExistingIP); 
	if (err == nil){
		t.Errorf("Scaling unittest check_for_green failed");
	}
}

func Test_check_for_nodeUp(t *testing.T) {
	err := ES_checkForNodeUp(NonExistingIP, NonExistingPOD, 5); 
	if (err == nil){
		t.Errorf("Scaling unittest check_for_nodeUp failed");
	}
}
