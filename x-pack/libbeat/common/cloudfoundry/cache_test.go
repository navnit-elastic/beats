// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

//go:build !integration

package cloudfoundry

import (
	"testing"
	"time"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/testing/testutils"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func TestClientCacheWrap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	testutils.SkipIfFIPSOnly(t, "cache uses SHA-1.")

	ttl := 2 * time.Second
	guid := mustCreateFakeGuid()
	app := cfclient.App{
		Guid: guid,
		Name: "Foo", // use this field to track if from cache or from client
	}
	fakeClient := &fakeCFClient{app, 0}
	logger := logptest.NewTestingLogger(t, "")
	cache, err := newClientCacheWrap(fakeClient, "test", ttl, ttl, logger.Named("cloudfoundry"))
	require.NoError(t, err)

	missingAppGuid := mustCreateFakeGuid()

	// should err; different app client doesn't have
	one, err := cache.GetAppByGuid(missingAppGuid)
	assert.Nil(t, one)
	assert.True(t, cfclient.IsAppNotFoundError(err))
	assert.Equal(t, 1, fakeClient.callCount)

	// calling again; the miss should be cached
	one, err = cache.GetAppByGuid(missingAppGuid)
	assert.Nil(t, one)
	assert.True(t, cfclient.IsAppNotFoundError(err))
	assert.Equal(t, 1, fakeClient.callCount)

	// fetched from client for the first time
	one, err = cache.GetAppByGuid(guid)
	assert.NoError(t, err)
	assert.Equal(t, app.Guid, one.Guid)
	assert.Equal(t, app.Name, one.Name)
	assert.Equal(t, 2, fakeClient.callCount)

	// updated app in fake client, new fetch should not have updated app
	updatedApp := cfclient.App{
		Guid: guid,
		Name: "Bar",
	}
	fakeClient.app = updatedApp
	two, err := cache.GetAppByGuid(guid)
	assert.NoError(t, err)
	assert.Equal(t, app.Guid, two.Guid)
	assert.Equal(t, app.Name, two.Name)
	assert.Equal(t, 2, fakeClient.callCount)

	// wait the ttl, then it should have updated app
	time.Sleep(ttl)
	three, err := cache.GetAppByGuid(guid)
	assert.NoError(t, err)
	assert.Equal(t, updatedApp.Guid, three.Guid)
	assert.Equal(t, updatedApp.Name, three.Name)
	assert.Equal(t, 3, fakeClient.callCount)
}

type fakeCFClient struct {
	app       cfclient.App
	callCount int
}

func (f *fakeCFClient) GetAppByGuid(guid string) (cfclient.App, error) {
	f.callCount++
	if f.app.Guid != guid {
		return cfclient.App{}, notFoundError()
	}
	return f.app, nil
}

func mustCreateFakeGuid() string {
	uuid, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}
	return uuid.String()
}

// notFoundError returns a cloud foundry error that satisfies cfclient.IsAppNotFoundError(err)
func notFoundError() error {
	return cfclient.CloudFoundryError{Code: 100004}
}
