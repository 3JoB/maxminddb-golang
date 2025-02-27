package maxminddb

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReader(t *testing.T) {
	for _, recordSize := range []uint{24, 28, 32} {
		for _, ipVersion := range []uint{4, 6} {
			fileName := fmt.Sprintf(
				testFile("MaxMind-DB-test-ipv%d-%d.mmdb"),
				ipVersion,
				recordSize,
			)
			reader, err := Open(fileName)
			require.NoError(t, err, "unexpected error while opening database: %v", err)
			checkMetadata(t, reader, ipVersion, recordSize)

			if ipVersion == 4 {
				checkIpv4(t, reader)
			} else {
				checkIpv6(t, reader)
			}
		}
	}
}

func TestReaderBytes(t *testing.T) {
	for _, recordSize := range []uint{24, 28, 32} {
		for _, ipVersion := range []uint{4, 6} {
			fileName := fmt.Sprintf(
				testFile("MaxMind-DB-test-ipv%d-%d.mmdb"),
				ipVersion,
				recordSize,
			)
			bytes, err := os.ReadFile(fileName)
			require.NoError(t, err)
			reader, err := FromBytes(bytes)
			require.NoError(t, err, "unexpected error while opening bytes: %v", err)

			checkMetadata(t, reader, ipVersion, recordSize)

			if ipVersion == 4 {
				checkIpv4(t, reader)
			} else {
				checkIpv6(t, reader)
			}
		}
	}
}

func TestLookupNetwork(t *testing.T) {
	bigInt := new(big.Int)
	bigInt.SetString("1329227995784915872903807060280344576", 10)
	decoderRecord := map[string]any{
		"array": []any{
			uint64(1),
			uint64(2),
			uint64(3),
		},
		"boolean": true,
		"bytes": []uint8{
			0x0,
			0x0,
			0x0,
			0x2a,
		},
		"double": 42.123456,
		"float":  float32(1.1),
		"int32":  -268435456,
		"map": map[string]any{
			"mapX": map[string]any{
				"arrayX": []any{
					uint64(0x7),
					uint64(0x8),
					uint64(0x9),
				},
				"utf8_stringX": "hello",
			},
		},
		"uint128":     bigInt,
		"uint16":      uint64(0x64),
		"uint32":      uint64(0x10000000),
		"uint64":      uint64(0x1000000000000000),
		"utf8_string": "unicode! ☯ - ♫",
	}

	tests := []struct {
		IP             net.IP
		DBFile         string
		ExpectedCIDR   string
		ExpectedRecord any
		ExpectedOK     bool
	}{
		{
			IP:             net.ParseIP("1.1.1.1"),
			DBFile:         "MaxMind-DB-test-ipv6-32.mmdb",
			ExpectedCIDR:   "1.0.0.0/8",
			ExpectedRecord: nil,
			ExpectedOK:     false,
		},
		{
			IP:             net.ParseIP("::1:ffff:ffff"),
			DBFile:         "MaxMind-DB-test-ipv6-24.mmdb",
			ExpectedCIDR:   "::1:ffff:ffff/128",
			ExpectedRecord: map[string]any{"ip": "::1:ffff:ffff"},
			ExpectedOK:     true,
		},
		{
			IP:             net.ParseIP("::2:0:1"),
			DBFile:         "MaxMind-DB-test-ipv6-24.mmdb",
			ExpectedCIDR:   "::2:0:0/122",
			ExpectedRecord: map[string]any{"ip": "::2:0:0"},
			ExpectedOK:     true,
		},
		{
			IP:             net.ParseIP("1.1.1.1"),
			DBFile:         "MaxMind-DB-test-ipv4-24.mmdb",
			ExpectedCIDR:   "1.1.1.1/32",
			ExpectedRecord: map[string]any{"ip": "1.1.1.1"},
			ExpectedOK:     true,
		},
		{
			IP:             net.ParseIP("1.1.1.3"),
			DBFile:         "MaxMind-DB-test-ipv4-24.mmdb",
			ExpectedCIDR:   "1.1.1.2/31",
			ExpectedRecord: map[string]any{"ip": "1.1.1.2"},
			ExpectedOK:     true,
		},
		{
			IP:             net.ParseIP("1.1.1.3"),
			DBFile:         "MaxMind-DB-test-decoder.mmdb",
			ExpectedCIDR:   "1.1.1.0/24",
			ExpectedRecord: decoderRecord,
			ExpectedOK:     true,
		},
		{
			IP:             net.ParseIP("::ffff:1.1.1.128"),
			DBFile:         "MaxMind-DB-test-decoder.mmdb",
			ExpectedCIDR:   "1.1.1.0/24",
			ExpectedRecord: decoderRecord,
			ExpectedOK:     true,
		},
		{
			IP:             net.ParseIP("::1.1.1.128"),
			DBFile:         "MaxMind-DB-test-decoder.mmdb",
			ExpectedCIDR:   "::101:100/120",
			ExpectedRecord: decoderRecord,
			ExpectedOK:     true,
		},
		{
			IP:             net.ParseIP("200.0.2.1"),
			DBFile:         "MaxMind-DB-no-ipv4-search-tree.mmdb",
			ExpectedCIDR:   "::/64",
			ExpectedRecord: "::0/64",
			ExpectedOK:     true,
		},
		{
			IP:             net.ParseIP("::200.0.2.1"),
			DBFile:         "MaxMind-DB-no-ipv4-search-tree.mmdb",
			ExpectedCIDR:   "::/64",
			ExpectedRecord: "::0/64",
			ExpectedOK:     true,
		},
		{
			IP:             net.ParseIP("0:0:0:0:ffff:ffff:ffff:ffff"),
			DBFile:         "MaxMind-DB-no-ipv4-search-tree.mmdb",
			ExpectedCIDR:   "::/64",
			ExpectedRecord: "::0/64",
			ExpectedOK:     true,
		},
		{
			IP:             net.ParseIP("ef00::"),
			DBFile:         "MaxMind-DB-no-ipv4-search-tree.mmdb",
			ExpectedCIDR:   "8000::/1",
			ExpectedRecord: nil,
			ExpectedOK:     false,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s - %s", test.DBFile, test.IP), func(t *testing.T) {
			var record any
			reader, err := Open(testFile(test.DBFile))
			require.NoError(t, err)

			network, ok, err := reader.LookupNetwork(test.IP, &record)
			require.NoError(t, err)
			assert.Equal(t, test.ExpectedOK, ok)
			assert.Equal(t, test.ExpectedCIDR, network.String())
			assert.Equal(t, test.ExpectedRecord, record)
		})
	}
}

func TestDecodingToInterface(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	require.NoError(t, err, "unexpected error while opening database: %v", err)

	var recordInterface any
	err = reader.Lookup(net.ParseIP("::1.1.1.0"), &recordInterface)
	require.NoError(t, err, "unexpected error while doing lookup: %v", err)

	checkDecodingToInterface(t, recordInterface)
}

func TestMetadataPointer(t *testing.T) {
	_, err := Open(testFile("MaxMind-DB-test-metadata-pointers.mmdb"))
	require.NoError(t, err, "unexpected error while opening database: %v", err)
}

func checkDecodingToInterface(t *testing.T, recordInterface any) {
	record := recordInterface.(map[string]any)
	assert.Equal(t, []any{uint64(1), uint64(2), uint64(3)}, record["array"])
	assert.Equal(t, true, record["boolean"])
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x2a}, record["bytes"])
	assert.Equal(t, 42.123456, record["double"])
	assert.Equal(t, float32(1.1), record["float"])
	assert.Equal(t, -268435456, record["int32"])
	assert.Equal(t,
		map[string]any{
			"mapX": map[string]any{
				"arrayX":       []any{uint64(7), uint64(8), uint64(9)},
				"utf8_stringX": "hello",
			},
		},
		record["map"],
	)

	assert.Equal(t, uint64(100), record["uint16"])
	assert.Equal(t, uint64(268435456), record["uint32"])
	assert.Equal(t, uint64(1152921504606846976), record["uint64"])
	assert.Equal(t, "unicode! ☯ - ♫", record["utf8_string"])
	bigInt := new(big.Int)
	bigInt.SetString("1329227995784915872903807060280344576", 10)
	assert.Equal(t, bigInt, record["uint128"])
}

type TestType struct {
	Array      []uint         `maxminddb:"array"`
	Boolean    bool           `maxminddb:"boolean"`
	Bytes      []byte         `maxminddb:"bytes"`
	Double     float64        `maxminddb:"double"`
	Float      float32        `maxminddb:"float"`
	Int32      int32          `maxminddb:"int32"`
	Map        map[string]any `maxminddb:"map"`
	Uint16     uint16         `maxminddb:"uint16"`
	Uint32     uint32         `maxminddb:"uint32"`
	Uint64     uint64         `maxminddb:"uint64"`
	Uint128    big.Int        `maxminddb:"uint128"`
	Utf8String string         `maxminddb:"utf8_string"`
}

func TestDecoder(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	require.NoError(t, err)

	verify := func(result TestType) {
		assert.Equal(t, []uint{uint(1), uint(2), uint(3)}, result.Array)
		assert.Equal(t, true, result.Boolean)
		assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x2a}, result.Bytes)
		assert.Equal(t, 42.123456, result.Double)
		assert.Equal(t, float32(1.1), result.Float)
		assert.Equal(t, int32(-268435456), result.Int32)

		assert.Equal(t,
			map[string]any{
				"mapX": map[string]any{
					"arrayX":       []any{uint64(7), uint64(8), uint64(9)},
					"utf8_stringX": "hello",
				},
			},
			result.Map,
		)

		assert.Equal(t, uint16(100), result.Uint16)
		assert.Equal(t, uint32(268435456), result.Uint32)
		assert.Equal(t, uint64(1152921504606846976), result.Uint64)
		assert.Equal(t, "unicode! ☯ - ♫", result.Utf8String)
		bigInt := new(big.Int)
		bigInt.SetString("1329227995784915872903807060280344576", 10)
		assert.Equal(t, bigInt, &result.Uint128)
	}

	{
		// Directly lookup and decode.
		var result TestType
		require.NoError(t, reader.Lookup(net.ParseIP("::1.1.1.0"), &result))
		verify(result)
	}
	{
		// Lookup record offset, then Decode.
		var result TestType
		offset, err := reader.LookupOffset(net.ParseIP("::1.1.1.0"))
		require.NoError(t, err)
		assert.NotEqual(t, NotFound, offset)

		assert.NoError(t, reader.Decode(offset, &result))
		verify(result)
	}

	assert.NoError(t, reader.Close())
}

type TestInterface interface {
	method() bool
}

func (t *TestType) method() bool {
	return t.Boolean
}

func TestStructInterface(t *testing.T) {
	var result TestInterface = &TestType{}

	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	require.NoError(t, err)

	require.NoError(t, reader.Lookup(net.ParseIP("::1.1.1.0"), &result))

	assert.Equal(t, true, result.method())
}

func TestNonEmptyNilInterface(t *testing.T) {
	var result TestInterface

	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	require.NoError(t, err)

	err = reader.Lookup(net.ParseIP("::1.1.1.0"), &result)
	assert.Equal(
		t,
		"maxminddb: cannot unmarshal map into type maxminddb.TestInterface",
		err.Error(),
	)
}

type CityTraits struct {
	AutonomousSystemNumber uint `json:"autonomous_system_number,omitempty" maxminddb:"autonomous_system_number"`
}

type City struct {
	Traits CityTraits `maxminddb:"traits"`
}

func TestEmbeddedStructAsInterface(t *testing.T) {
	var city City
	var result any = city.Traits

	db, err := Open(testFile("GeoIP2-ISP-Test.mmdb"))
	require.NoError(t, err)

	assert.NoError(t, db.Lookup(net.ParseIP("1.128.0.0"), &result))
}

type BoolInterface interface {
	true() bool
}

type Bool bool

func (b Bool) true() bool {
	return bool(b)
}

type ValueTypeTestType struct {
	Boolean BoolInterface `maxminddb:"boolean"`
}

func TestValueTypeInterface(t *testing.T) {
	var result ValueTypeTestType
	result.Boolean = Bool(false)

	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	require.NoError(t, err)

	// although it would be nice to support cases like this, I am not sure it
	// is possible to do so in a general way.
	assert.Error(t, reader.Lookup(net.ParseIP("::1.1.1.0"), &result))
}

type NestedMapX struct {
	UTF8StringX string `maxminddb:"utf8_stringX"`
}

type NestedPointerMapX struct {
	ArrayX []int `maxminddb:"arrayX"`
}

type PointerMap struct {
	MapX struct {
		Ignored string
		NestedMapX
		*NestedPointerMapX
	} `maxminddb:"mapX"`
}

type TestPointerType struct {
	Array   *[]uint     `maxminddb:"array"`
	Boolean *bool       `maxminddb:"boolean"`
	Bytes   *[]byte     `maxminddb:"bytes"`
	Double  *float64    `maxminddb:"double"`
	Float   *float32    `maxminddb:"float"`
	Int32   *int32      `maxminddb:"int32"`
	Map     *PointerMap `maxminddb:"map"`
	Uint16  *uint16     `maxminddb:"uint16"`
	Uint32  *uint32     `maxminddb:"uint32"`

	// Test for pointer to pointer
	Uint64 **uint64 `maxminddb:"uint64"`

	Uint128    *big.Int `maxminddb:"uint128"`
	Utf8String *string  `maxminddb:"utf8_string"`
}

func TestComplexStructWithNestingAndPointer(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	assert.NoError(t, err)

	var result TestPointerType

	err = reader.Lookup(net.ParseIP("::1.1.1.0"), &result)
	require.NoError(t, err)

	assert.Equal(t, []uint{uint(1), uint(2), uint(3)}, *result.Array)
	assert.Equal(t, true, *result.Boolean)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x2a}, *result.Bytes)
	assert.Equal(t, 42.123456, *result.Double)
	assert.Equal(t, float32(1.1), *result.Float)
	assert.Equal(t, int32(-268435456), *result.Int32)

	assert.Equal(t, []int{7, 8, 9}, result.Map.MapX.ArrayX)

	assert.Equal(t, "hello", result.Map.MapX.UTF8StringX)

	assert.Equal(t, uint16(100), *result.Uint16)
	assert.Equal(t, uint32(268435456), *result.Uint32)
	assert.Equal(t, uint64(1152921504606846976), **result.Uint64)
	assert.Equal(t, "unicode! ☯ - ♫", *result.Utf8String)
	bigInt := new(big.Int)
	bigInt.SetString("1329227995784915872903807060280344576", 10)
	assert.Equal(t, bigInt, result.Uint128)

	assert.NoError(t, reader.Close())
}

// See GitHub #115.
func TestNestedMapDecode(t *testing.T) {
	db, err := Open(testFile("GeoIP2-Country-Test.mmdb"))
	require.NoError(t, err)

	var r map[string]map[string]any

	require.NoError(t, db.Lookup(net.ParseIP("89.160.20.128"), &r))

	assert.Equal(
		t,
		map[string]map[string]any{
			"continent": {
				"code":       "EU",
				"geoname_id": uint64(6255148),
				"names": map[string]any{
					"de":    "Europa",
					"en":    "Europe",
					"es":    "Europa",
					"fr":    "Europe",
					"ja":    "ヨーロッパ",
					"pt-BR": "Europa",
					"ru":    "Европа",
					"zh-CN": "欧洲",
				},
			},
			"country": {
				"geoname_id":           uint64(2661886),
				"is_in_european_union": true,
				"iso_code":             "SE",
				"names": map[string]any{
					"de":    "Schweden",
					"en":    "Sweden",
					"es":    "Suecia",
					"fr":    "Suède",
					"ja":    "スウェーデン王国",
					"pt-BR": "Suécia",
					"ru":    "Швеция",
					"zh-CN": "瑞典",
				},
			},
			"registered_country": {
				"geoname_id":           uint64(2921044),
				"is_in_european_union": true,
				"iso_code":             "DE",
				"names": map[string]any{
					"de":    "Deutschland",
					"en":    "Germany",
					"es":    "Alemania",
					"fr":    "Allemagne",
					"ja":    "ドイツ連邦共和国",
					"pt-BR": "Alemanha",
					"ru":    "Германия",
					"zh-CN": "德国",
				},
			},
		},
		r,
	)
}

func TestNestedOffsetDecode(t *testing.T) {
	db, err := Open(testFile("GeoIP2-City-Test.mmdb"))
	require.NoError(t, err)

	off, err := db.LookupOffset(net.ParseIP("81.2.69.142"))
	assert.NotEqual(t, off, NotFound)
	require.NoError(t, err)

	var root struct {
		CountryOffset uintptr `maxminddb:"country"`

		Location struct {
			Latitude float64 `maxminddb:"latitude"`
			// Longitude is directly nested within the parent map.
			LongitudeOffset uintptr `maxminddb:"longitude"`
			// TimeZone is indirected via a pointer.
			TimeZoneOffset uintptr `maxminddb:"time_zone"`
		} `maxminddb:"location"`
	}
	assert.NoError(t, db.Decode(off, &root))
	assert.Equal(t, 51.5142, root.Location.Latitude)

	var longitude float64
	assert.NoError(t, db.Decode(root.Location.LongitudeOffset, &longitude))
	assert.Equal(t, -0.0931, longitude)

	var timeZone string
	assert.NoError(t, db.Decode(root.Location.TimeZoneOffset, &timeZone))
	assert.Equal(t, "Europe/London", timeZone)

	var country struct {
		IsoCode string `maxminddb:"iso_code"`
	}
	assert.NoError(t, db.Decode(root.CountryOffset, &country))
	assert.Equal(t, "GB", country.IsoCode)

	assert.NoError(t, db.Close())
}

func TestDecodingUint16IntoInt(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	require.NoError(t, err, "unexpected error while opening database: %v", err)

	var result struct {
		Uint16 int `maxminddb:"uint16"`
	}
	err = reader.Lookup(net.ParseIP("::1.1.1.0"), &result)
	require.NoError(t, err)

	assert.Equal(t, 100, result.Uint16)
}

func TestIpv6inIpv4(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-ipv4-24.mmdb"))
	require.NoError(t, err, "unexpected error while opening database: %v", err)

	var result TestType
	err = reader.Lookup(net.ParseIP("2001::"), &result)

	var emptyResult TestType
	assert.Equal(t, emptyResult, result)

	expected := errors.New(
		"error looking up '2001::': you attempted to look up an IPv6 address in an IPv4-only database",
	)
	assert.Equal(t, expected, err)
	assert.NoError(t, reader.Close(), "error on close")
}

func TestBrokenDoubleDatabase(t *testing.T) {
	reader, err := Open(testFile("GeoIP2-City-Test-Broken-Double-Format.mmdb"))
	require.NoError(t, err, "unexpected error while opening database: %v", err)

	var result any
	err = reader.Lookup(net.ParseIP("2001:220::"), &result)

	expected := newInvalidDatabaseError(
		"the MaxMind DB file's data section contains bad data (float 64 size of 2)",
	)
	assert.Equal(t, expected, err)
	assert.NoError(t, reader.Close(), "error on close")
}

func TestInvalidNodeCountDatabase(t *testing.T) {
	_, err := Open(testFile("GeoIP2-City-Test-Invalid-Node-Count.mmdb"))

	expected := newInvalidDatabaseError("the MaxMind DB contains invalid metadata")
	assert.Equal(t, expected, err)
}

func TestMissingDatabase(t *testing.T) {
	reader, err := Open("file-does-not-exist.mmdb")
	assert.Nil(t, reader, "received reader when doing lookups on DB that doesn't exist")
	assert.Regexp(t, "open file-does-not-exist.mmdb.*", err)
}

func TestNonDatabase(t *testing.T) {
	reader, err := Open("README.md")
	assert.Nil(t, reader, "received reader when doing lookups on DB that doesn't exist")
	assert.Equal(t, "error opening database: invalid MaxMind DB file", err.Error())
}

func TestDecodingToNonPointer(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	require.NoError(t, err)

	var recordInterface any
	err = reader.Lookup(net.ParseIP("::1.1.1.0"), recordInterface)
	assert.Equal(t, "result param must be a pointer", err.Error())
	assert.NoError(t, reader.Close(), "error on close")
}

func TestNilLookup(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	require.NoError(t, err)

	var recordInterface any
	err = reader.Lookup(nil, recordInterface)
	assert.Equal(t, "IP passed to Lookup cannot be nil", err.Error())
	assert.NoError(t, reader.Close(), "error on close")
}

func TestUsingClosedDatabase(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	var recordInterface any

	err = reader.Lookup(nil, recordInterface)
	assert.Equal(t, "cannot call Lookup on a closed database", err.Error())

	_, err = reader.LookupOffset(nil)
	assert.Equal(t, "cannot call LookupOffset on a closed database", err.Error())

	err = reader.Decode(0, recordInterface)
	assert.Equal(t, "cannot call Decode on a closed database", err.Error())
}

func checkMetadata(t *testing.T, reader *Reader, ipVersion, recordSize uint) {
	metadata := reader.Metadata

	assert.Equal(t, uint(2), metadata.BinaryFormatMajorVersion)

	assert.Equal(t, uint(0), metadata.BinaryFormatMinorVersion)
	assert.IsType(t, uint(0), metadata.BuildEpoch)
	assert.Equal(t, "Test", metadata.DatabaseType)

	assert.Equal(t, metadata.Description,
		map[string]string{
			"en": "Test Database",
			"zh": "Test Database Chinese",
		})
	assert.Equal(t, ipVersion, metadata.IPVersion)
	assert.Equal(t, []string{"en", "zh"}, metadata.Languages)

	if ipVersion == 4 {
		assert.Equal(t, uint(164), metadata.NodeCount)
	} else {
		assert.Equal(t, uint(416), metadata.NodeCount)
	}

	assert.Equal(t, recordSize, metadata.RecordSize)
}

func checkIpv4(t *testing.T, reader *Reader) {
	for i := uint(0); i < 6; i++ {
		address := fmt.Sprintf("1.1.1.%d", uint(1)<<i)
		ip := net.ParseIP(address)

		var result map[string]string
		err := reader.Lookup(ip, &result)
		assert.NoError(t, err, "unexpected error while doing lookup: %v", err)
		assert.Equal(t, map[string]string{"ip": address}, result)
	}
	pairs := map[string]string{
		"1.1.1.3":  "1.1.1.2",
		"1.1.1.5":  "1.1.1.4",
		"1.1.1.7":  "1.1.1.4",
		"1.1.1.9":  "1.1.1.8",
		"1.1.1.15": "1.1.1.8",
		"1.1.1.17": "1.1.1.16",
		"1.1.1.31": "1.1.1.16",
	}

	for keyAddress, valueAddress := range pairs {
		data := map[string]string{"ip": valueAddress}

		ip := net.ParseIP(keyAddress)

		var result map[string]string
		err := reader.Lookup(ip, &result)
		assert.NoError(t, err, "unexpected error while doing lookup: %v", err)
		assert.Equal(t, data, result)
	}

	for _, address := range []string{"1.1.1.33", "255.254.253.123"} {
		ip := net.ParseIP(address)

		var result map[string]string
		err := reader.Lookup(ip, &result)
		assert.NoError(t, err, "unexpected error while doing lookup: %v", err)
		assert.Nil(t, result)
	}
}

func checkIpv6(t *testing.T, reader *Reader) {
	subnets := []string{
		"::1:ffff:ffff", "::2:0:0",
		"::2:0:40", "::2:0:50", "::2:0:58",
	}

	for _, address := range subnets {
		var result map[string]string
		err := reader.Lookup(net.ParseIP(address), &result)
		assert.NoError(t, err, "unexpected error while doing lookup: %v", err)
		assert.Equal(t, map[string]string{"ip": address}, result)
	}

	pairs := map[string]string{
		"::2:0:1":  "::2:0:0",
		"::2:0:33": "::2:0:0",
		"::2:0:39": "::2:0:0",
		"::2:0:41": "::2:0:40",
		"::2:0:49": "::2:0:40",
		"::2:0:52": "::2:0:50",
		"::2:0:57": "::2:0:50",
		"::2:0:59": "::2:0:58",
	}

	for keyAddress, valueAddress := range pairs {
		data := map[string]string{"ip": valueAddress}
		var result map[string]string
		err := reader.Lookup(net.ParseIP(keyAddress), &result)
		assert.NoError(t, err, "unexpected error while doing lookup: %v", err)
		assert.Equal(t, data, result)
	}

	for _, address := range []string{"1.1.1.33", "255.254.253.123", "89fa::"} {
		var result map[string]string
		err := reader.Lookup(net.ParseIP(address), &result)
		assert.NoError(t, err, "unexpected error while doing lookup: %v", err)
		assert.Nil(t, result)
	}
}

func BenchmarkOpen(b *testing.B) {
	var db *Reader
	var err error
	for i := 0; i < b.N; i++ {
		db, err = Open("GeoLite2-City.mmdb")
		if err != nil {
			b.Error(err)
		}
	}
	assert.NotNil(b, db)
	assert.NoError(b, db.Close(), "error on close")
}

func BenchmarkInterfaceLookup(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	require.NoError(b, err)

	//nolint:gosec // this is a test
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var result any

	ip := make(net.IP, 4)
	for i := 0; i < b.N; i++ {
		randomIPv4Address(r, ip)
		err = db.Lookup(ip, &result)
		if err != nil {
			b.Error(err)
		}
	}
	assert.NoError(b, db.Close(), "error on close")
}

func BenchmarkInterfaceLookupNetwork(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	require.NoError(b, err)

	//nolint:gosec // this is a test
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var result any

	ip := make(net.IP, 4)
	for i := 0; i < b.N; i++ {
		randomIPv4Address(r, ip)
		_, _, err = db.LookupNetwork(ip, &result)
		if err != nil {
			b.Error(err)
		}
	}
	assert.NoError(b, db.Close(), "error on close")
}

type fullCity struct {
	City struct {
		GeoNameID uint              `maxminddb:"geoname_id"`
		Names     map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Continent struct {
		Code      string            `maxminddb:"code"`
		GeoNameID uint              `maxminddb:"geoname_id"`
		Names     map[string]string `maxminddb:"names"`
	} `maxminddb:"continent"`
	Country struct {
		GeoNameID         uint              `maxminddb:"geoname_id"`
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
		IsoCode           string            `maxminddb:"iso_code"`
		Names             map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Location struct {
		AccuracyRadius uint16  `maxminddb:"accuracy_radius"`
		Latitude       float64 `maxminddb:"latitude"`
		Longitude      float64 `maxminddb:"longitude"`
		MetroCode      uint    `maxminddb:"metro_code"`
		TimeZone       string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
	Postal struct {
		Code string `maxminddb:"code"`
	} `maxminddb:"postal"`
	RegisteredCountry struct {
		GeoNameID         uint              `maxminddb:"geoname_id"`
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
		IsoCode           string            `maxminddb:"iso_code"`
		Names             map[string]string `maxminddb:"names"`
	} `maxminddb:"registered_country"`
	RepresentedCountry struct {
		GeoNameID         uint              `maxminddb:"geoname_id"`
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
		IsoCode           string            `maxminddb:"iso_code"`
		Names             map[string]string `maxminddb:"names"`
		Type              string            `maxminddb:"type"`
	} `maxminddb:"represented_country"`
	Subdivisions []struct {
		GeoNameID uint              `maxminddb:"geoname_id"`
		IsoCode   string            `maxminddb:"iso_code"`
		Names     map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	Traits struct {
		IsAnonymousProxy    bool `maxminddb:"is_anonymous_proxy"`
		IsSatelliteProvider bool `maxminddb:"is_satellite_provider"`
	} `maxminddb:"traits"`
}

func BenchmarkCityLookup(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	require.NoError(b, err)

	//nolint:gosec // this is a test
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var result fullCity

	ip := make(net.IP, 4)
	for i := 0; i < b.N; i++ {
		randomIPv4Address(r, ip)
		err = db.Lookup(ip, &result)
		if err != nil {
			b.Error(err)
		}
	}
	assert.NoError(b, db.Close(), "error on close")
}

func BenchmarkCityLookupNetwork(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	require.NoError(b, err)

	//nolint:gosec // this is a test
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var result fullCity

	ip := make(net.IP, 4)
	for i := 0; i < b.N; i++ {
		randomIPv4Address(r, ip)
		_, _, err = db.LookupNetwork(ip, &result)
		if err != nil {
			b.Error(err)
		}
	}
	assert.NoError(b, db.Close(), "error on close")
}

func BenchmarkCountryCode(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	require.NoError(b, err)

	type MinCountry struct {
		Country struct {
			IsoCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}

	//nolint:gosec // this is a test
	r := rand.New(rand.NewSource(0))
	var result MinCountry

	ip := make(net.IP, 4)
	for i := 0; i < b.N; i++ {
		randomIPv4Address(r, ip)
		err = db.Lookup(ip, &result)
		if err != nil {
			b.Error(err)
		}
	}
	assert.NoError(b, db.Close(), "error on close")
}

func randomIPv4Address(r *rand.Rand, ip []byte) {
	num := r.Uint32()
	ip[0] = byte(num >> 24)
	ip[1] = byte(num >> 16)
	ip[2] = byte(num >> 8)
	ip[3] = byte(num)
}

func testFile(file string) string {
	return filepath.Join("test-data", "test-data", file)
}
