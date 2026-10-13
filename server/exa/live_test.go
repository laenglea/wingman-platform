package exa_test

import (
	"testing"

	"github.com/adrianliechti/wingman/test/webtest"
)

func TestLiveHTTP(t *testing.T) { webtest.LiveHTTP(t, "exa") }
func TestLiveSDK(t *testing.T)  { webtest.LiveSDK(t, "exa") }
