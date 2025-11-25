package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
)

func TestDatabaseConfig(t *testing.T) {
	assert.Equal(t, "my-host", DatabaseConfig{Host: "my-host"}.GetHostname())
	assert.Equal(t, "my-host:1234", DatabaseConfig{Host: "my-host", Port: 1234}.GetHostname())
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		c   Config
		err string
	}{
		{Config{Links: []*wfv1.Link{{URL: "javascript:foo"}}}, "protocol javascript is not allowed"},
		{Config{Links: []*wfv1.Link{{URL: "javASCRipt: //foo"}}}, "protocol javascript is not allowed"},
		{Config{Links: []*wfv1.Link{{URL: "http://foo.bar/?foo=<script>abc</script>bar"}}}, ""},
	}
	for _, tt := range tests {
		err := tt.c.Sanitize([]string{"http", "https"})
		if tt.err != "" {
			require.EqualError(t, err, tt.err)
		} else {
			require.NoError(t, err)
		}
	}
}

func TestFailedPodRestartConfig_IsEnabled(t *testing.T) {
	// nil config should return false
	var nilConfig *FailedPodRestartConfig
	assert.False(t, nilConfig.IsEnabled())

	// empty config should return false
	emptyConfig := &FailedPodRestartConfig{}
	assert.False(t, emptyConfig.IsEnabled())

	// enabled config should return true
	enabledConfig := &FailedPodRestartConfig{Enabled: true}
	assert.True(t, enabledConfig.IsEnabled())
}

func TestFailedPodRestartConfig_GetMaxRestarts(t *testing.T) {
	// nil config should return default of 3
	var nilConfig *FailedPodRestartConfig
	assert.Equal(t, int32(3), nilConfig.GetMaxRestarts())

	// config with nil MaxRestarts should return default of 3
	configNoMax := &FailedPodRestartConfig{Enabled: true}
	assert.Equal(t, int32(3), configNoMax.GetMaxRestarts())

	// config with MaxRestarts should return that value
	configWithMax := &FailedPodRestartConfig{
		Enabled:     true,
		MaxRestarts: ptr.To(int32(5)),
	}
	assert.Equal(t, int32(5), configWithMax.GetMaxRestarts())

	// config with MaxRestarts of 0 should return 0
	configZeroMax := &FailedPodRestartConfig{
		Enabled:     true,
		MaxRestarts: ptr.To(int32(0)),
	}
	assert.Equal(t, int32(0), configZeroMax.GetMaxRestarts())
}

func TestFailedPodRestartConfig_GetBackoffDuration(t *testing.T) {
	// nil config should return default of 30s
	var nilConfig *FailedPodRestartConfig
	assert.Equal(t, 30*time.Second, nilConfig.GetBackoffDuration())

	// config with nil BackoffDuration should return default of 30s
	configNoBackoff := &FailedPodRestartConfig{Enabled: true}
	assert.Equal(t, 30*time.Second, configNoBackoff.GetBackoffDuration())

	// config with BackoffDuration should return that value
	configWithBackoff := &FailedPodRestartConfig{
		Enabled:         true,
		BackoffDuration: &metav1.Duration{Duration: 60 * time.Second},
	}
	assert.Equal(t, 60*time.Second, configWithBackoff.GetBackoffDuration())
}
