package volumes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestErrVolumeAlreadyDefined(t *testing.T) {
	err := NewErrVolumeAlreadyDefined("test-path")
	assert.EqualError(t, err, `volume for container path "test-path" is already defined`)
}

func TestNewDefaultManager(t *testing.T) {
	logger := common.NewBuildLogger(nil, nil)

	m := NewManager(logger, nil, ManagerConfig{})
	assert.IsType(t, &manager{}, m)
}

func newDefaultManager(config ManagerConfig) *manager {
	m := &manager{
		logger:         common.NewBuildLogger(nil, nil),
		config:         config,
		managedVolumes: make(map[string]bool, 0),
	}

	return m
}

func addContainerManager(manager *manager) *MockContainerManager {
	containerManager := new(MockContainerManager)

	manager.containerManager = containerManager

	return containerManager
}

func TestDefaultManager_CreateUserVolumes_HostVolume(t *testing.T) {
	testCases := map[string]struct {
		volume          string
		fullProjectDir  string
		expectedBinding []string
		expectedError   error
	}{
		"no volumes specified": {
			volume:          "",
			expectedBinding: []string{"/host:/duplicated"},
		},
		"volume with absolute path": {
			volume:          "/host:/volume",
			expectedBinding: []string{"/host:/duplicated", "/host:/volume"},
		},
		"volume with absolute path and with fullProjectDir specified": {
			volume:          "/host:/volume",
			fullProjectDir:  "/builds",
			expectedBinding: []string{"/host:/duplicated", "/host:/volume"},
		},
		"volume without absolute path and without fullProjectDir specified": {
			volume:          "/host:volume",
			expectedBinding: []string{"/host:/duplicated", "/host:volume"},
		},
		"volume without absolute path and with fullProjectDir specified": {
			volume:          "/host:volume",
			fullProjectDir:  "/builds/project",
			expectedBinding: []string{"/host:/duplicated", "/host:/builds/project/volume"},
		},
		"duplicated volume specification": {
			volume:          "/host/new:/duplicated",
			expectedBinding: []string{"/host:/duplicated"},
			expectedError:   NewErrVolumeAlreadyDefined("/duplicated"),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				FullProjectDir: testCase.fullProjectDir,
			}

			m := newDefaultManager(config)

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			err = m.Create(testCase.volume)
			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, testCase.expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_Disabled(t *testing.T) {
	testCases := map[string]struct {
		volume         string
		fullProjectDir string
		disableCache   bool

		expectedBinding           []string
		expectedCacheContainerIDs []string
		expectedConfigVolume      string
		expectedError             error
	}{
		"no volumes specified": {
			volume:          "",
			expectedBinding: []string{"/host:/duplicated"},
		},
		"volume with absolute path, without fullProjectDir and with disableCache": {
			volume:          "/volume",
			fullProjectDir:  "",
			disableCache:    true,
			expectedBinding: []string{"/host:/duplicated"},
		},
		"volume with absolute path, with fullProjectDir and with disableCache": {
			volume:          "/volume",
			fullProjectDir:  "/builds/project",
			disableCache:    true,
			expectedBinding: []string{"/host:/duplicated"},
		},
		"volume without absolute path, without fullProjectDir and with disableCache": {
			volume:          "volume",
			disableCache:    true,
			expectedBinding: []string{"/host:/duplicated"},
		},
		"volume without absolute path, with fullProjectDir and with disableCache": {
			volume:          "volume",
			fullProjectDir:  "/builds/project",
			disableCache:    true,
			expectedBinding: []string{"/host:/duplicated"},
		},
		"duplicated volume with absolute path, without fullProjectDir and with disableCache": {
			volume:          "/duplicated",
			fullProjectDir:  "",
			disableCache:    true,
			expectedBinding: []string{"/host:/duplicated"},
			expectedError:   NewErrVolumeAlreadyDefined("/duplicated"),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				FullProjectDir: testCase.fullProjectDir,
				DisableCache:   testCase.disableCache,
			}

			m := newDefaultManager(config)

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			err = m.Create(testCase.volume)
			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, testCase.expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_HostBased(t *testing.T) {
	testCases := map[string]struct {
		volume          string
		fullProjectDir  string
		disableCache    bool
		cacheDir        string
		projectUniqName string

		expectedBinding           string
		expectedCacheContainerIDs []string
		expectedConfigVolume      string
	}{
		"volume with absolute path, without fullProjectDir, without disableCache and with cacheDir": {
			volume:          "/volume",
			disableCache:    false,
			cacheDir:        "/cache",
			projectUniqName: "project-uniq",
			expectedBinding: "/cache/project-uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume",
		},
		"volume with absolute path, with fullProjectDir, without disableCache and with cacheDir": {
			volume:          "/volume",
			fullProjectDir:  "/builds/project",
			disableCache:    false,
			cacheDir:        "/cache",
			projectUniqName: "project-uniq",
			expectedBinding: "/cache/project-uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume",
		},
		"volume without absolute path, without fullProjectDir, without disableCache and with cacheDir": {
			volume:          "volume",
			disableCache:    false,
			cacheDir:        "/cache",
			projectUniqName: "project-uniq",
			expectedBinding: "/cache/project-uniq/210ab9e731c9c36c2c38db15c28a8d1c:volume",
		},
		"volume without absolute path, with fullProjectDir, without disableCache and with cacheDir": {
			volume:          "volume",
			fullProjectDir:  "/builds/project",
			disableCache:    false,
			cacheDir:        "/cache",
			projectUniqName: "project-uniq",
			expectedBinding: "/cache/project-uniq/f69aef9fb01e88e6213362a04877452d:/builds/project/volume",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				FullProjectDir:  testCase.fullProjectDir,
				DisableCache:    testCase.disableCache,
				CacheDir:        testCase.cacheDir,
				ProjectUniqName: testCase.projectUniqName,
			}

			m := newDefaultManager(config)

			err := m.Create(testCase.volume)
			assert.NoError(t, err)
			assertVolumeBindings(t, testCase.expectedBinding, m.volumeBindings)
		})
	}
}

func assertVolumeBindings(t *testing.T, expectedBinding string, bindings []string) {
	if expectedBinding == "" {
		return
	}
	assert.Contains(t, bindings, expectedBinding)

}

func TestDefaultManager_CreateUserVolumes_CacheVolume_ContainerBased(t *testing.T) {
	testCases := map[string]struct {
		volume                   string
		fullProjectDir           string
		projectUniqName          string
		expectedContainerName    string
		expectedContainerPath    string
		existingContainerID      string
		newContainerID           string
		expectedCacheContainerID string
	}{
		"volume with absolute path, without fullProjectDir and with existing container": {
			volume:                   "/volume",
			fullProjectDir:           "",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa",
			expectedContainerPath:    "/volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume with absolute path, without fullProjectDir and with new container": {
			volume:                   "/volume",
			fullProjectDir:           "",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa",
			expectedContainerPath:    "/volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
		"volume without absolute path, without fullProjectDir and with existing container": {
			volume:                   "volume",
			fullProjectDir:           "",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-210ab9e731c9c36c2c38db15c28a8d1c",
			expectedContainerPath:    "volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume without absolute path, without fullProjectDir and with new container": {
			volume:                   "volume",
			fullProjectDir:           "",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-210ab9e731c9c36c2c38db15c28a8d1c",
			expectedContainerPath:    "volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
		"volume without absolute path, with fullProjectDir and with existing container": {
			volume:                   "volume",
			fullProjectDir:           "/builds/project",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-f69aef9fb01e88e6213362a04877452d",
			expectedContainerPath:    "/builds/project/volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume without absolute path, with fullProjectDir and with new container": {
			volume:                   "volume",
			fullProjectDir:           "/builds/project",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-f69aef9fb01e88e6213362a04877452d",
			expectedContainerPath:    "/builds/project/volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				FullProjectDir:  testCase.fullProjectDir,
				ProjectUniqName: testCase.projectUniqName,
			}

			m := newDefaultManager(config)
			containerManager := addContainerManager(m)

			defer containerManager.AssertExpectations(t)

			containerManager.On("FindExistingCacheContainer", testCase.expectedContainerName, testCase.expectedContainerPath).
				Return(testCase.existingContainerID).
				Once()

			if testCase.newContainerID != "" {
				containerManager.On("CreateCacheContainer", testCase.expectedContainerName, testCase.expectedContainerPath).
					Return(testCase.newContainerID, nil).
					Once()
			}

			err := m.Create(testCase.volume)
			assert.NoError(t, err)

			assert.Contains(t, m.cacheContainerIDs, testCase.expectedCacheContainerID)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_ContainerBased_WithError(t *testing.T) {
	config := ManagerConfig{
		FullProjectDir:  "/builds/project",
		ProjectUniqName: "project-uniq",
	}

	m := newDefaultManager(config)
	containerManager := addContainerManager(m)

	defer containerManager.AssertExpectations(t)

	containerManager.On("FindExistingCacheContainer", "project-uniq-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
		Return("").
		Once()

	containerManager.On("CreateCacheContainer", "project-uniq-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
		Return("", errors.New("test error")).
		Once()

	err := m.Create("volume")
	assert.Error(t, err)
}

func TestDefaultManager_CreateBuildVolume_WithoutError(t *testing.T) {
	testCases := map[string]struct {
		jobsRootDir           string
		volumes               []string
		gitStrategy           common.GitStrategy
		disableCache          bool
		cacheDir              string
		projectUniqName       string
		expectedContainerName string
		expectedContainerPath string
		newContainerID        string
		expectedError         error
		expectedBinding       string
		expectedTmpAndCacheID string
	}{
		"build directory within host mounted volumes": {
			jobsRootDir: "/builds/root",
			volumes:     []string{"/host/builds:/builds"},
		},
		"persistent cache container": {
			jobsRootDir:     "/builds/root",
			gitStrategy:     common.GitFetch,
			disableCache:    false,
			cacheDir:        "/cache",
			projectUniqName: "project-uniq",
			expectedBinding: "/cache/project-uniq/28934d7b9a9154212a5dd671e4fa5704:/builds/root",
		},
		"temporary cache container": {
			jobsRootDir:           "/builds/root",
			gitStrategy:           common.GitClone,
			expectedContainerName: "",
			expectedContainerPath: "/builds/root",
			newContainerID:        "newContainerID",
			expectedTmpAndCacheID: "newContainerID",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				GitStrategy:     testCase.gitStrategy,
				DisableCache:    testCase.disableCache,
				CacheDir:        testCase.cacheDir,
				ProjectUniqName: testCase.projectUniqName,
			}

			m := newDefaultManager(config)
			containerManager := addContainerManager(m)

			defer containerManager.AssertExpectations(t)

			if testCase.expectedContainerPath != "" {
				containerManager.On("CreateCacheContainer", testCase.expectedContainerName, testCase.expectedContainerPath).
					Return(testCase.newContainerID, nil).
					Once()
			}

			err := m.CreateBuildVolume(testCase.jobsRootDir, testCase.volumes)
			if testCase.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, testCase.expectedError.Error())
			}

			if testCase.expectedBinding != "" {
				assertVolumeBindings(t, testCase.expectedBinding, m.volumeBindings)
			}

			if testCase.expectedTmpAndCacheID != "" {
				assert.Contains(t, m.cacheContainerIDs, testCase.expectedTmpAndCacheID)
				assert.Contains(t, m.tmpContainerIDs, testCase.expectedTmpAndCacheID)
			}
		})
	}
}

func TestDefaultManager_CreateBuildVolume_WithError(t *testing.T) {
	config := ManagerConfig{
		GitStrategy: common.GitClone,
	}

	m := newDefaultManager(config)
	containerManager := addContainerManager(m)

	defer containerManager.AssertExpectations(t)

	containerManager.On("CreateCacheContainer", "", "/builds/root").
		Return("", errors.New("test error")).
		Once()

	err := m.CreateBuildVolume("/builds/root", []string{"/host/source:/destination"})
	assert.Error(t, err)
}

func TestDefaultManager_Binds(t *testing.T) {
	expectedElements := []string{"element1", "element2"}
	m := &manager{
		volumeBindings: expectedElements,
	}

	assert.Equal(t, expectedElements, m.Binds())
}

func TestDefaultManager_ContainerIDs(t *testing.T) {
	expectedElements := []string{"element1", "element2"}
	m := &manager{
		cacheContainerIDs: expectedElements,
	}

	assert.Equal(t, expectedElements, m.ContainerIDs())
}

func TestDefaultManager_Cleanup(t *testing.T) {
	cManager := new(MockContainerManager)
	defer cManager.AssertExpectations(t)

	cManager.On("RemoveCacheContainer", mock.Anything, "container-1").
		Return(nil).
		Once()

	m := &manager{
		containerManager: cManager,
		tmpContainerIDs:  []string{"container-1"},
	}

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)

	done := m.Cleanup(ctx)
	<-done
}
