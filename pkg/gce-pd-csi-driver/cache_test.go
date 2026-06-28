package gceGCEDriver

import (
	"errors"
	"testing"
)

func TestFetchChunkSizeKiB(t *testing.T) {
	testCases := []struct {
		name         string
		cacheSize    string
		expChunkSize string
		expErr       bool
	}{
		{
			name:         "chunk size is in the allowed range",
			cacheSize:    "500GiB",
			expChunkSize: "512KiB", //range defined in fetchChunkSizeKiB
		},
		{
			name:         "chunk size is set to the range ceil",
			cacheSize:    "30000000GiB",
			expChunkSize: "1048576KiB", //range defined in fetchChunkSizeKiB - max 1GiB
		},
		{
			name:         "chunk size is set to the allowed range floor",
			cacheSize:    "100GiB",
			expChunkSize: "160KiB", //range defined in fetchChunkSizeKiB - min 160 KiB
		},
		{
			name:         "cacheSize set to KiB also sets the chunk size to range floor",
			cacheSize:    "1GiB",
			expChunkSize: "160KiB", //range defined in fetchChunkSizeKiB - min 160 KiB
		},
		{
			name:         "chunk size with GiB string parses correctly",
			cacheSize:    "375GiB",
			expChunkSize: "384KiB",
		},
		{
			name:         "invalid cacheSize",
			cacheSize:    "fdfsdKi",
			expChunkSize: "160KiB", //range defined in fetchChunkSizeKiB - min 160 KiB
			expErr:       true,
		},
		// cacheSize is validated in storage class parameter so assuming invalid cacheSize (like negative, 0) would not be passed to the function
	}

	for _, tc := range testCases {
		chunkSize, err := fetchChunkSizeKiB(tc.cacheSize)
		if err != nil {
			if !tc.expErr {
				t.Errorf("Errored %s", err)
			}
			continue
		}
		if chunkSize != tc.expChunkSize {
			t.Errorf("Got %s want %s", chunkSize, tc.expChunkSize)
		}

	}

}

func TestFetchNumberGiB(t *testing.T) {
	testCases := []struct {
		name        string
		stringInput []string
		expOutput   string // Outputs value in GiB
		expErr      bool
	}{
		{
			name:        "valid input 1",
			stringInput: []string{"5000000000B"},
			expOutput:   "5GiB", //range defined in fetchChunkSizeKiB
		},
		{
			name:        "valid input 2",
			stringInput: []string{"375000000000B"}, // 1 LSSD attached
			expOutput:   "350GiB",                  //range defined in fetchChunkSizeKiB
		},
		{
			name:        "valid input 3",
			stringInput: []string{"9000000000000B"}, // 24 LSSD attached
			expOutput:   "8382GiB",                  //range defined in fetchChunkSizeKiB
		},
		{
			name:        "valid input 4",
			stringInput: []string{"Some text before ", "9000000000000B", "Some text after"}, // 24 LSSD attached
			expOutput:   "8382GiB",                                                          //range defined in fetchChunkSizeKiB
		},
		{
			name:        "invalid input 1",
			stringInput: []string{"9000000000000"},
			expErr:      true,
		},
		{
			name:        "invalid input 2",
			stringInput: []string{"A9000000000000B"},
			expErr:      true,
		},
		{
			name:        "valid input 5",
			stringInput: []string{"900000B"}, // <1GiB gets rounded off to 0GiB
			expOutput:   "1GiB",
		},
	}

	for _, tc := range testCases {
		v, err := fetchNumberGiB(tc.stringInput)
		if err != nil {
			if !tc.expErr {
				t.Errorf("Errored %s", err)
			}
			continue
		}
		if v != tc.expOutput {
			t.Errorf("Got %s want %s", v, tc.expOutput)
		}

	}

}

func TestIsValidVGName(t *testing.T) {
	testCases := []struct {
		name     string
		vgName   string
		expected bool
	}{
		{
			name:     "valid simple name",
			vgName:   "csi-vg-nndw98vv",
			expected: true,
		},
		{
			name:     "valid name with all characters",
			vgName:   "a-z_A-Z_0-9_._-_+",
			expected: true,
		},
		{
			name:     "empty name",
			vgName:   "",
			expected: false,
		},
		{
			name:     "invalid name with spaces",
			vgName:   "csi vg nndw98vv",
			expected: false,
		},
		{
			name:     "invalid name with warning prefix",
			vgName:   "WARNING: VG csi-vg-nndw98vv is missing PV",
			expected: false,
		},
		{
			name:     "invalid name with slash",
			vgName:   "/dev/md127",
			expected: false,
		},
		{
			name:     "invalid name with parenthesis",
			vgName:   "md127)",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isValidVGName(tc.vgName)
			if actual != tc.expected {
				t.Errorf("isValidVGName(%q) = %v; want %v", tc.vgName, actual, tc.expected)
			}
		})
	}
}

func TestClassifyCacheState(t *testing.T) {
	boom := errors.New("lvs exit status 5")
	testCases := []struct {
		name       string
		output     string
		runErr     error
		wantCached bool
		wantErr    bool
	}{
		{
			name:       "cached, clean exit",
			output:     "  pvc-123-" + cacheSuffix + "_cpool\n",
			runErr:     nil,
			wantCached: true,
		},
		{
			name:       "cached, warned-and-exited-nonzero on missing PV",
			output:     "  WARNING: VG is missing PV\n  pvc-123-" + cacheSuffix + "_cpool\n",
			runErr:     boom,
			wantCached: true,
		},
		{
			name:       "linear volume, clean exit, empty pool_lv",
			output:     "  \n",
			runErr:     nil,
			wantCached: false,
		},
		{
			name:       "lv genuinely absent, clean exit, no rows",
			output:     "",
			runErr:     nil,
			wantCached: false,
		},
		{
			name:    "unknown state: command failed with no output -> fail closed",
			output:  "",
			runErr:  boom,
			wantErr: true,
		},
		{
			name:    "unknown state: only whitespace output on failure -> fail closed",
			output:  "   \n",
			runErr:  boom,
			wantErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cached, err := classifyCacheState([]byte(tc.output), tc.runErr)
			if (err != nil) != tc.wantErr {
				t.Fatalf("classifyCacheState err = %v; wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && cached != tc.wantCached {
				t.Errorf("classifyCacheState cached = %v; want %v", cached, tc.wantCached)
			}
		})
	}
}
