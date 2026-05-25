//go:generate go run ./cmd/generate

package main

import (
	"fmt"
	"os"
	"strconv"

	"frisi/ha/entities"

	ga "saml.dev/gome-assistant"
)

func pvEinspeisungColor(svc *ga.Service, state ga.State, data ga.EntityData) {
	watts, err := strconv.ParseFloat(data.ToState, 64)
	if err != nil {
		return
	}
	if err := applyPVColor(svc.Light, state, entities.Light.LightPv, entities.Zone.HomeZone, watts); err != nil {
		fmt.Printf("pv automation error: %v\n", err)
	}
}

func main() {
	app, err := ga.NewApp(ga.NewAppRequest{
		URL:              os.Getenv("HA_URL"),
		HAAuthToken:      os.Getenv("HA_AUTH_TOKEN"),
		HomeZoneEntityId: entities.Zone.HomeZone,
	})
	if err != nil {
		fmt.Printf("error creating app: %v\n", err)
		os.Exit(1)
	}
	defer app.Cleanup()

	app.RegisterEntityListeners(
		ga.NewEntityListener().
			EntityIds(entities.Sensor.EnergyGridPower).
			Call(pvEinspeisungColor).
			Build(),
	)

	app.Start()
}
