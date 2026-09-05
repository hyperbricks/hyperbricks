package main

import (
	"fmt"
	"sync"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
)

func TestRouteConfigAndPlanArePublishedTogether(t *testing.T) {
	configMutex.Lock()
	oldConfigs := configs
	oldRoutePlans := routePlans
	configMutex.Unlock()
	t.Cleanup(func() {
		updateGlobalRoutes(oldConfigs, oldRoutePlans)
	})

	planA := &renderplan.Plan{}
	planB := &renderplan.Plan{}
	configsA := map[string]map[string]interface{}{
		"index": {"version": "A"},
	}
	configsB := map[string]map[string]interface{}{
		"index": {"version": "B"},
	}
	plansA := map[string]*renderplan.Plan{"index": planA}
	plansB := map[string]*renderplan.Plan{"index": planB}
	updateGlobalRoutes(configsA, plansA)

	const publications = 10_000
	const readers = 8
	stop := make(chan struct{})
	errors := make(chan error, readers)
	var wait sync.WaitGroup
	for range readers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				config, plan, found := getConfigAndPlan("index")
				if !found {
					errors <- fmt.Errorf("published route is missing")
					return
				}
				switch config["version"] {
				case "A":
					if plan != planA {
						errors <- fmt.Errorf("config A was paired with another plan")
						return
					}
				case "B":
					if plan != planB {
						errors <- fmt.Errorf("config B was paired with another plan")
						return
					}
				default:
					errors <- fmt.Errorf("unexpected route version %v", config["version"])
					return
				}
			}
		}()
	}

	for index := 0; index < publications; index++ {
		if index%2 == 0 {
			updateGlobalRoutes(configsB, plansB)
		} else {
			updateGlobalRoutes(configsA, plansA)
		}
	}
	close(stop)
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
