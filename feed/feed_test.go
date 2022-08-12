package feed

import "testing"

//
// Test helpers
//

func assertClosed(t *testing.T, relChan chan *Release, errChan chan error) {
	var ok bool
	for i := 0; i < 10; i++ {
		_, ok = <-relChan
		if !ok {
			break
		}
	}
	if ok {
		t.Errorf("expected release channel to be closed")
	}
	for i := 0; i < 10; i++ {
		_, ok = <-errChan
		if !ok {
			break
		}
	}
	if ok {
		t.Errorf("expected error channel to be closed")
	}
}
