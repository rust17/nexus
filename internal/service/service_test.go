package service

import (
	"context"
	"nexus/internal/balancer"
	"nexus/internal/config"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestService(t *testing.T) {
	// Test configuration
	cfg := &config.ServiceConfig{
		Name:         "test-service",
		BalancerType: "round_robin",
		Servers: []config.ServerConfig{
			{Address: "server1:8080", Weight: 1},
			{Address: "server2:8080", Weight: 1},
		},
	}

	t.Run("TestName", func(t *testing.T) {
		s := NewService(cfg)
		assert.Equal(t, "test-service", s.Name())
	})

	t.Run("TestNextServer", func(t *testing.T) {
		s := NewService(cfg)
		ctx := context.Background()

		// Test polling logic
		addr1, _ := s.NextServer(ctx)
		addr2, _ := s.NextServer(ctx)
		assert.NotEqual(t, addr1, addr2)
	})

	t.Run("TestBalancer", func(t *testing.T) {
		s := NewService(cfg)
		b := s.Balancer()
		assert.IsType(t, &balancer.RoundRobinBalancer{}, b)
	})

	t.Run("TestUpdate", func(t *testing.T) {
		s := NewService(cfg)
		newCfg := &config.ServiceConfig{
			Name:         "updated-service",
			BalancerType: "weighted_round_robin",
			Servers: []config.ServerConfig{
				{Address: "server3:8080", Weight: 2},
			},
		}

		err := s.Update(newCfg)
		assert.Nil(t, err)
		assert.Equal(t, "updated-service", s.Name())
		assert.IsType(t, &balancer.WeightedRoundRobinBalancer{}, s.Balancer())
	})

	t.Run("TestConcurrentUpdate", func(t *testing.T) {
		s := NewService(cfg)
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				newCfg := &config.ServiceConfig{
					Name:         "concurrent-update",
					BalancerType: "round_robin",
					Servers:      cfg.Servers,
				}
				_ = s.Update(newCfg)
			}()
		}
		wg.Wait()
		assert.Equal(t, "concurrent-update", s.Name())
	})
}
