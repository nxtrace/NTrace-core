package dn42

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindPtrRecordMatchesCity(t *testing.T) {
	dir := t.TempDir()
	ptrPath := filepath.Join(dir, "ptr.csv")
	content := "HKG,hk,Hong Kong,Hong Kong\nLAX,us,California,Los Angeles\n"
	require.NoError(t, os.WriteFile(ptrPath, []byte(content), 0o644))

	viper.Set("ptrPath", ptrPath)
	t.Cleanup(viper.Reset)

	row, err := FindPtrRecord("core.hongkong-1.example")
	require.NoError(t, err)

	assert.Equal(t, "hk", row.LtdCode)
	assert.Equal(t, "Hong Kong", row.Region)
	assert.Equal(t, "Hong Kong", row.City)
}

func TestFindPtrRecordMatchesIATACode(t *testing.T) {
	dir := t.TempDir()
	ptrPath := filepath.Join(dir, "ptr.csv")
	require.NoError(t, os.WriteFile(ptrPath, []byte("LAX,us,California,Los Angeles\n"), 0o644))

	viper.Set("ptrPath", ptrPath)
	t.Cleanup(viper.Reset)

	row, err := FindPtrRecord("edge.lax01.provider.test")
	require.NoError(t, err)

	assert.Equal(t, "lax", row.IATACode)
	assert.Equal(t, "us", row.LtdCode)
	assert.Equal(t, "California", row.Region)
}

func TestFindPtrRecordNotFound(t *testing.T) {
	dir := t.TempDir()
	ptrPath := filepath.Join(dir, "ptr.csv")
	require.NoError(t, os.WriteFile(ptrPath, []byte(""), 0o644))

	viper.Set("ptrPath", ptrPath)
	t.Cleanup(viper.Reset)

	_, err := FindPtrRecord("unmatched.example")
	require.Error(t, err)
}
