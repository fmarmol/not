package not

import (
	"os"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	c := Config{}

	err := toml.NewEncoder(os.Stdout).Encode(c)
	require.NoError(t, err)

}
