package main

import (
	"errors"
	"testing"
	"time"

	ga "saml.dev/gome-assistant"
)

const testLightID = "light.pv_lampe_licht"
const testZoneID  = "zone.home"

// --- mocks ---

type mockLight struct {
	calls        []mockLightCall
	turnOffCalls []string
	err          error
	notifyOff    chan struct{} // closed on first TurnOff, for timer tests
}

type mockLightCall struct {
	entityID    string
	serviceData map[string]any
}

func (m *mockLight) TurnOff(entityId string) error {
	m.turnOffCalls = append(m.turnOffCalls, entityId)
	if m.notifyOff != nil {
		close(m.notifyOff)
		m.notifyOff = nil
	}
	return m.err
}

func (m *mockLight) TurnOn(entityId string, serviceData ...map[string]any) error {
	call := mockLightCall{entityID: entityId}
	if len(serviceData) > 0 {
		call.serviceData = serviceData[0]
	}
	m.calls = append(m.calls, call)
	return m.err
}

type mockState struct {
	states map[string]string
	err    error
}

func (m *mockState) Get(entityId string) (ga.EntityState, error) {
	if m.err != nil {
		return ga.EntityState{}, m.err
	}
	s, ok := m.states[entityId]
	if !ok {
		return ga.EntityState{}, errors.New("entity not found: " + entityId)
	}
	return ga.EntityState{EntityID: entityId, State: s}, nil
}

func stateWith(id, state string) *mockState {
	return &mockState{states: map[string]string{id: state}}
}

// --- wattToRGB ---

func TestWattToRGB(t *testing.T) {
	tests := []struct {
		watts float64
		wantR int // approximate expected channel (just direction check)
		desc  string
	}{
		{0, 255, "0W should be red (R max)"},
		{2000, 0, "2000W should be green (R=0)"},
		{2001, 0, "above max clamps to green"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			rgb := wattToRGB(tt.watts)
			if rgb[0] != tt.wantR {
				t.Errorf("wattToRGB(%v) R = %d, want %d", tt.watts, rgb[0], tt.wantR)
			}
		})
	}
}

func TestWattToRGB_MidpointIsYellow(t *testing.T) {
	rgb := wattToRGB(1000)
	// At 50% (hue ~60°) we expect greenish-yellow: both R and G high, B low
	if rgb[0] == 0 || rgb[1] == 0 {
		t.Errorf("wattToRGB(1000) = %v, expected R and G both non-zero", rgb)
	}
	if rgb[2] > 10 {
		t.Errorf("wattToRGB(1000) B = %d, expected near 0", rgb[2])
	}
}

// --- hslToRgb ---

func TestHslToRgb_Achromatic(t *testing.T) {
	rgb := hslToRgb(0, 0, 0.5)
	if rgb[0] != 127 || rgb[1] != 127 || rgb[2] != 127 {
		t.Errorf("hslToRgb(0,0,0.5) = %v, want [127 127 127]", rgb)
	}
}

func TestHslToRgb_Red(t *testing.T) {
	rgb := hslToRgb(0, 1, 0.5)
	if rgb[0] != 255 || rgb[1] != 0 || rgb[2] != 0 {
		t.Errorf("hslToRgb(0,1,0.5) = %v, want [255 0 0]", rgb)
	}
}

func TestHslToRgb_Green(t *testing.T) {
	rgb := hslToRgb(1.0/3, 1, 0.5)
	if rgb[0] != 0 || rgb[1] != 255 || rgb[2] != 0 {
		t.Errorf("hslToRgb(1/3,1,0.5) = %v, want [0 255 0]", rgb)
	}
}

// --- applyPVColor ---

func TestApplyPVColor_NoFeedIn(t *testing.T) {
	light := &mockLight{}
	for _, watts := range []float64{0, -1, -100} {
		light.calls = nil
		if err := applyPVColor(light, stateWith(testZoneID, "1"), testLightID, testZoneID, watts); err != nil {
			t.Errorf("watts=%v: unexpected error: %v", watts, err)
		}
		if len(light.calls) != 0 {
			t.Errorf("watts=%v: expected no light call, got %d", watts, len(light.calls))
		}
	}
}

func TestApplyPVColor_NobodyHome(t *testing.T) {
	light := &mockLight{}
	if err := applyPVColor(light, stateWith(testZoneID, "0"), testLightID, testZoneID, 500); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(light.calls) != 0 {
		t.Errorf("expected no light call when nobody home, got %d", len(light.calls))
	}
}

func TestApplyPVColor_SomebodyHome(t *testing.T) {
	light := &mockLight{}
	if err := applyPVColor(light, stateWith(testZoneID, "1"), testLightID, testZoneID, 500); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(light.calls) != 1 {
		t.Fatalf("expected 1 light call, got %d", len(light.calls))
	}
	call := light.calls[0]
	if call.entityID != testLightID {
		t.Errorf("called wrong entity: %s", call.entityID)
	}
	if call.serviceData["brightness_pct"] != 1 {
		t.Errorf("brightness_pct = %v, want 1", call.serviceData["brightness_pct"])
	}
	rgb, ok := call.serviceData["rgb_color"].([]int)
	if !ok || len(rgb) != 3 {
		t.Errorf("rgb_color invalid: %v", call.serviceData["rgb_color"])
	}
}

func TestApplyPVColor_ZoneStateError(t *testing.T) {
	light := &mockLight{}
	state := &mockState{err: errors.New("HA unavailable")}
	err := applyPVColor(light, state, testLightID, testZoneID, 500)
	if err == nil {
		t.Fatal("expected error when zone state unavailable")
	}
	if len(light.calls) != 0 {
		t.Errorf("expected no light call on state error")
	}
}

func TestApplyPVColor_MaxWattClampsToGreen(t *testing.T) {
	light := &mockLight{}
	if err := applyPVColor(light, stateWith(testZoneID, "2"), testLightID, testZoneID, 9999); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rgb := light.calls[0].serviceData["rgb_color"].([]int)
	if rgb[0] != 0 {
		t.Errorf("R = %d at max watts, want 0 (green)", rgb[0])
	}
}

// --- pvOffAutomation ---

func TestPVOff_HighFeedIn(t *testing.T) {
	light := &mockLight{}
	pvOff := newPVOffAutomation(testLightID, 0)
	pvOff.onStateChange(light, "500")
	time.Sleep(10 * time.Millisecond)
	if len(light.turnOffCalls) != 0 {
		t.Errorf("expected no TurnOff for high watts, got %d", len(light.turnOffCalls))
	}
}

func TestPVOff_LowFeedIn_TurnsOff(t *testing.T) {
	done := make(chan struct{})
	light := &mockLight{notifyOff: done}
	pvOff := newPVOffAutomation(testLightID, 0)
	pvOff.onStateChange(light, "0")
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("TurnOff not called within timeout")
	}
	if light.turnOffCalls[0] != testLightID {
		t.Errorf("TurnOff called on wrong entity: %s", light.turnOffCalls[0])
	}
}

func TestPVOff_LowThenHigh_CancelsTimer(t *testing.T) {
	light := &mockLight{}
	pvOff := newPVOffAutomation(testLightID, 50*time.Millisecond)
	pvOff.onStateChange(light, "0")   // start timer
	pvOff.onStateChange(light, "500") // cancel before it fires
	time.Sleep(100 * time.Millisecond)
	if len(light.turnOffCalls) != 0 {
		t.Errorf("expected TurnOff cancelled, but it was called %d time(s)", len(light.turnOffCalls))
	}
}

func TestPVOff_MultipleLow_OnlyOneTimer(t *testing.T) {
	done := make(chan struct{})
	light := &mockLight{notifyOff: done}
	pvOff := newPVOffAutomation(testLightID, 10*time.Millisecond)
	pvOff.onStateChange(light, "0") // starts timer
	pvOff.onStateChange(light, "0") // timer already running, no second timer
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("TurnOff not called within timeout")
	}
	if len(light.turnOffCalls) != 1 {
		t.Errorf("expected exactly 1 TurnOff, got %d", len(light.turnOffCalls))
	}
}

func TestPVOff_NonNumericState(t *testing.T) {
	light := &mockLight{}
	pvOff := newPVOffAutomation(testLightID, 0)
	pvOff.onStateChange(light, "unavailable") // must not panic
	time.Sleep(10 * time.Millisecond)
	if len(light.turnOffCalls) != 0 {
		t.Errorf("expected no call for non-numeric state")
	}
}
