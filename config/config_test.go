package config_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/config"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/google/go-cmp/cmp"
)

type testStruct struct {
	String            string              `default:"Ya_Code"`
	Int               int                 `default:"42"`
	Int8              int8                `default:"84"`
	Int16             int16               `default:"168"`
	Int32             int32               `default:"336"`
	Int64             int64               `default:"672"`
	Uint              uint                `default:"84"`
	Uint8             uint8               `default:"168"`
	Uint16            uint16              `default:"336"`
	Uint32            uint32              `default:"672"`
	Uint64            uint64              `default:"1344"`
	Float             float64             `default:"3.14"`
	Float32           float32             `default:"1.618"`
	Float64           float64             `default:"2.718"`
	Bool              bool                `default:"true"`
	Bytes             []byte              `default:"1,2,3"`
	IntSlice          []int               `default:"1,2,3"`
	Int8Slice         []int8              `default:"4,5,6"`
	Int16Slice        []int16             `default:"7,8,9"`
	Int32Slice        []int32             `default:"10,11,12"`
	Int64Slice        []int64             `default:"13,14,15"`
	UintSlice         []uint              `default:"16,17,18"`
	Uint8Slice        []uint8             `default:"19,20,21"`
	Uint16Slice       []uint16            `default:"22,23,24"`
	Uint32Slice       []uint32            `default:"25,26,27"`
	Uint64Slice       []uint64            `default:"28,29,30"`
	FloatSlice        []float64           `default:"31.1,32.2,33.3"`
	Float32Slice      []float32           `default:"34.4,35.5,36.6"`
	Float64Slice      []float64           `default:"37.7,38.8,39.9"`
	BoolSlice         []bool              `default:"true,false,true"`
	StringSlice       []string            `default:"Ya_Code,Skalse,Oleksandr,Vasab1tch,Olderestin"`
	MapStringString   map[string]string   `default:"yarozpach:OctaviaZilber"`
	MapStringInt      map[string]int      `default:"yashluha:1,anzhelchikk:2"`
	MapStringInt8     map[string]int8     `default:"foo:1,bar:2"`
	MapStringInt16    map[string]int16    `default:"foo:1,bar:2"`
	MapStringInt32    map[string]int32    `default:"foo:1,bar:2"`
	MapStringInt64    map[string]int64    `default:"foo:1,bar:2"`
	MapStringUint     map[string]uint     `default:"foo:1,bar:2"`
	MapStringUint8    map[string]uint8    `default:"foo:1,bar:2"`
	MapStringUint16   map[string]uint16   `default:"foo:1,bar:2"`
	MapStringUint32   map[string]uint32   `default:"foo:1,bar:2"`
	MapStringUint64   map[string]uint64   `default:"foo:1,bar:2"`
	MapStringFloat32  map[string]float32  `default:"foo:1.1,bar:2.2"`
	MapStringFloat64  map[string]float64  `default:"foo:1.1,bar:2.2"`
	MapStringBool     map[string]bool     `default:"foo:true,bar:false"`
	MapIntInt         map[int]int         `default:"-1:100,2:200,3:300"`
	MapIntInt8        map[int]int8        `default:"-1:10,2:20,3:30"`
	MapIntInt16       map[int]int16       `default:"-1:11,2:22,3:33"`
	MapIntInt32       map[int]int32       `default:"-1:111,2:222,3:333"`
	MapIntInt64       map[int]int64       `default:"-1:1111,2:2222,3:3333"`
	MapIntUint        map[int]uint        `default:"-1:1000,2:2000,3:3000"`
	MapIntUint8       map[int]uint8       `default:"-1:10,2:20,3:30"`
	MapIntUint16      map[int]uint16      `default:"-1:10000,2:20000,3:30000"`
	MapIntUint32      map[int]uint32      `default:"-1:100000,2:200000,3:300000"`
	MapIntUint64      map[int]uint64      `default:"-1:1000000,2:2000000,3:3000000"`
	MapIntFloat32     map[int]float32     `default:"-1:1.1,2:2.2,3:3.3"`
	MapIntFloat64     map[int]float64     `default:"-1:111.1,2:222.2,3:333.3"`
	MapIntBool        map[int]bool        `default:"-1:true,2:false,3:true"`
	MapIntString      map[int]string      `default:"-1:foo,2:bar"`
	MapInt8Int        map[int8]int        `default:"-1:100,2:200,3:300"`
	MapInt8Int8       map[int8]int8       `default:"-1:10,2:20,3:30"`
	MapInt8Int16      map[int8]int16      `default:"-1:11,2:22,3:33"`
	MapInt8Int32      map[int8]int32      `default:"-1:111,2:222,3:333"`
	MapInt8Int64      map[int8]int64      `default:"-1:1111,2:2222,3:3333"`
	MapInt8Uint       map[int8]uint       `default:"-1:1000,2:2000,3:3000"`
	MapInt8Uint8      map[int8]uint8      `default:"-1:10,2:20,3:30"`
	MapInt8Uint16     map[int8]uint16     `default:"-1:10000,2:20000,3:30000"`
	MapInt8Uint32     map[int8]uint32     `default:"-1:100000,2:200000,3:300000"`
	MapInt8Uint64     map[int8]uint64     `default:"-1:1000000,2:2000000,3:3000000"`
	MapInt8Float32    map[int8]float32    `default:"-1:1.1,2:2.2,3:3.3"`
	MapInt8Float64    map[int8]float64    `default:"-1:111.1,2:222.2,3:333.3"`
	MapInt8Bool       map[int8]bool       `default:"-1:true,2:false,3:true"`
	MapInt8String     map[int8]string     `default:"-1:foo,2:bar"`
	MapInt16Int       map[int16]int       `default:"-1:100,2:200,3:300"`
	MapInt16Int8      map[int16]int8      `default:"-1:10,2:20,3:30"`
	MapInt16Int16     map[int16]int16     `default:"-1:11,2:22,3:33"`
	MapInt16Int32     map[int16]int32     `default:"-1:111,2:222,3:333"`
	MapInt16Int64     map[int16]int64     `default:"-1:1111,2:2222,3:3333"`
	MapInt16Uint      map[int16]uint      `default:"-1:1000,2:2000,3:3000"`
	MapInt16Uint8     map[int16]uint8     `default:"-1:10,2:20,3:30"`
	MapInt16Uint16    map[int16]uint16    `default:"-1:10000,2:20000,3:30000"`
	MapInt16Uint32    map[int16]uint32    `default:"-1:100000,2:200000,3:300000"`
	MapInt16Uint64    map[int16]uint64    `default:"-1:1000000,2:2000000,3:3000000"`
	MapInt16Float32   map[int16]float32   `default:"-1:1.1,2:2.2,3:3.3"`
	MapInt16Float64   map[int16]float64   `default:"-1:111.1,2:222.2,3:333.3"`
	MapInt16Bool      map[int16]bool      `default:"-1:true,2:false,3:true"`
	MapInt16String    map[int16]string    `default:"-1:foo,2:bar"`
	MapInt32Int       map[int32]int       `default:"-1:100,2:200,3:300"`
	MapInt32Int8      map[int32]int8      `default:"-1:10,2:20,3:30"`
	MapInt32Int16     map[int32]int16     `default:"-1:11,2:22,3:33"`
	MapInt32Int32     map[int32]int32     `default:"-1:111,2:222,3:333"`
	MapInt32Int64     map[int32]int64     `default:"-1:1111,2:2222,3:3333"`
	MapInt32Uint      map[int32]uint      `default:"-1:1000,2:2000,3:3000"`
	MapInt32Uint8     map[int32]uint8     `default:"-1:10,2:20,3:30"`
	MapInt32Uint16    map[int32]uint16    `default:"-1:10000,2:20000,3:30000"`
	MapInt32Uint32    map[int32]uint32    `default:"-1:100000,2:200000,3:300000"`
	MapInt32Uint64    map[int32]uint64    `default:"-1:1000000,2:2000000,3:3000000"`
	MapInt32Float32   map[int32]float32   `default:"-1:1.1,2:2.2,3:3.3"`
	MapInt32Float64   map[int32]float64   `default:"-1:111.1,2:222.2,3:333.3"`
	MapInt32Bool      map[int32]bool      `default:"-1:true,2:false,3:true"`
	MapInt32String    map[int32]string    `default:"-1:foo,2:bar"`
	MapInt64Int       map[int64]int       `default:"-1:100,2:200,3:300"`
	MapInt64Int8      map[int64]int8      `default:"-1:10,2:20,3:30"`
	MapInt64Int16     map[int64]int16     `default:"-1:11,2:22,3:33"`
	MapInt64Int32     map[int64]int32     `default:"-1:111,2:222,3:333"`
	MapInt64Int64     map[int64]int64     `default:"-1:1111,2:2222,3:3333"`
	MapInt64Uint      map[int64]uint      `default:"-1:1000,2:2000,3:3000"`
	MapInt64Uint8     map[int64]uint8     `default:"-1:10,2:20,3:30"`
	MapInt64Uint16    map[int64]uint16    `default:"-1:10000,2:20000,3:30000"`
	MapInt64Uint32    map[int64]uint32    `default:"-1:100000,2:200000,3:300000"`
	MapInt64Uint64    map[int64]uint64    `default:"-1:1000000,2:2000000,3:3000000"`
	MapInt64Float32   map[int64]float32   `default:"-1:1.1,2:2.2,3:3.3"`
	MapInt64Float64   map[int64]float64   `default:"-1:111.1,2:222.2,3:333.3"`
	MapInt64Bool      map[int64]bool      `default:"-1:true,2:false,3:true"`
	MapInt64String    map[int64]string    `default:"-1:foo,2:bar"`
	MapUintInt        map[uint]int        `default:"1:100,2:200,3:300"`
	MapUintInt8       map[uint]int8       `default:"1:10,2:20,3:30"`
	MapUintInt16      map[uint]int16      `default:"1:11,2:22,3:33"`
	MapUintInt32      map[uint]int32      `default:"1:111,2:222,3:333"`
	MapUintInt64      map[uint]int64      `default:"1:1111,2:2222,3:3333"`
	MapUintUint       map[uint]uint       `default:"1:1000,2:2000,3:3000"`
	MapUintUint8      map[uint]uint8      `default:"1:10,2:20,3:30"`
	MapUintUint16     map[uint]uint16     `default:"1:10000,2:20000,3:30000"`
	MapUintUint32     map[uint]uint32     `default:"1:100000,2:200000,3:300000"`
	MapUintUint64     map[uint]uint64     `default:"1:1000000,2:2000000,3:3000000"`
	MapUintFloat32    map[uint]float32    `default:"1:1.1,2:2.2,3:3.3"`
	MapUintFloat64    map[uint]float64    `default:"1:111.1,2:222.2,3:333.3"`
	MapUintBool       map[uint]bool       `default:"1:true,2:false,3:true"`
	MapUintString     map[uint]string     `default:"1:foo,2:bar"`
	MapUint8Int       map[uint8]int       `default:"1:100,2:200,3:300"`
	MapUint8Int8      map[uint8]int8      `default:"1:10,2:20,3:30"`
	MapUint8Int16     map[uint8]int16     `default:"1:11,2:22,3:33"`
	MapUint8Int32     map[uint8]int32     `default:"1:111,2:222,3:333"`
	MapUint8Int64     map[uint8]int64     `default:"1:1111,2:2222,3:3333"`
	MapUint8Uint      map[uint8]uint      `default:"1:1000,2:2000,3:3000"`
	MapUint8Uint8     map[uint8]uint8     `default:"1:10,2:20,3:30"`
	MapUint8Uint16    map[uint8]uint16    `default:"1:10000,2:20000,3:30000"`
	MapUint8Uint32    map[uint8]uint32    `default:"1:100000,2:200000,3:300000"`
	MapUint8Uint64    map[uint8]uint64    `default:"1:1000000,2:2000000,3:3000000"`
	MapUint8Float32   map[uint8]float32   `default:"1:1.1,2:2.2,3:3.3"`
	MapUint8Float64   map[uint8]float64   `default:"1:111.1,2:222.2,3:333.3"`
	MapUint8Bool      map[uint8]bool      `default:"1:true,2:false,3:true"`
	MapUint8String    map[uint8]string    `default:"1:foo,2:bar"`
	MapUint16Int      map[uint16]int      `default:"1:100,2:200,3:300"`
	MapUint16Int8     map[uint16]int8     `default:"1:10,2:20,3:30"`
	MapUint16Int16    map[uint16]int16    `default:"1:11,2:22,3:33"`
	MapUint16Int32    map[uint16]int32    `default:"1:111,2:222,3:333"`
	MapUint16Int64    map[uint16]int64    `default:"1:1111,2:2222,3:3333"`
	MapUint16Uint     map[uint16]uint     `default:"1:1000,2:2000,3:3000"`
	MapUint16Uint8    map[uint16]uint8    `default:"1:10,2:20,3:30"`
	MapUint16Uint16   map[uint16]uint16   `default:"1:10000,2:20000,3:30000"`
	MapUint16Uint32   map[uint16]uint32   `default:"1:100000,2:200000,3:300000"`
	MapUint16Uint64   map[uint16]uint64   `default:"1:1000000,2:2000000,3:3000000"`
	MapUint16Float32  map[uint16]float32  `default:"1:1.1,2:2.2,3:3.3"`
	MapUint16Float64  map[uint16]float64  `default:"1:111.1,2:222.2,3:333.3"`
	MapUint16Bool     map[uint16]bool     `default:"1:true,2:false,3:true"`
	MapUint16String   map[uint16]string   `default:"1:foo,2:bar"`
	MapUint32Int      map[uint32]int      `default:"1:100,2:200,3:300"`
	MapUint32Int8     map[uint32]int8     `default:"1:10,2:20,3:30"`
	MapUint32Int16    map[uint32]int16    `default:"1:11,2:22,3:33"`
	MapUint32Int32    map[uint32]int32    `default:"1:111,2:222,3:333"`
	MapUint32Int64    map[uint32]int64    `default:"1:1111,2:2222,3:3333"`
	MapUint32Uint     map[uint32]uint     `default:"1:1000,2:2000,3:3000"`
	MapUint32Uint8    map[uint32]uint8    `default:"1:10,2:20,3:30"`
	MapUint32Uint16   map[uint32]uint16   `default:"1:10000,2:20000,3:30000"`
	MapUint32Uint32   map[uint32]uint32   `default:"1:100000,2:200000,3:300000"`
	MapUint32Uint64   map[uint32]uint64   `default:"1:1000000,2:2000000,3:3000000"`
	MapUint32Float32  map[uint32]float32  `default:"1:1.1,2:2.2,3:3.3"`
	MapUint32Float64  map[uint32]float64  `default:"1:111.1,2:222.2,3:333.3"`
	MapUint32Bool     map[uint32]bool     `default:"1:true,2:false,3:true"`
	MapUint32String   map[uint32]string   `default:"1:foo,2:bar"`
	MapUint64Int      map[uint64]int      `default:"1:100,2:200,3:300"`
	MapUint64Int8     map[uint64]int8     `default:"1:10,2:20,3:30"`
	MapUint64Int16    map[uint64]int16    `default:"1:11,2:22,3:33"`
	MapUint64Int32    map[uint64]int32    `default:"1:111,2:222,3:333"`
	MapUint64Int64    map[uint64]int64    `default:"1:1111,2:2222,3:3333"`
	MapUint64Uint     map[uint64]uint     `default:"1:1000,2:2000,3:3000"`
	MapUint64Uint8    map[uint64]uint8    `default:"1:10,2:20,3:30"`
	MapUint64Uint16   map[uint64]uint16   `default:"1:10000,2:20000,3:30000"`
	MapUint64Uint32   map[uint64]uint32   `default:"1:100000,2:200000,3:300000"`
	MapUint64Uint64   map[uint64]uint64   `default:"1:1000000,2:2000000,3:3000000"`
	MapUint64Float32  map[uint64]float32  `default:"1:1.1,2:2.2,3:3.3"`
	MapUint64Float64  map[uint64]float64  `default:"1:111.1,2:222.2,3:333.3"`
	MapUint64Bool     map[uint64]bool     `default:"1:true,2:false,3:true"`
	MapUint64String   map[uint64]string   `default:"1:foo,2:bar"`
	MapFloat32Int     map[float32]int     `default:"1:100,2:200,3:300"`
	MapFloat32Int8    map[float32]int8    `default:"1:10,2:20,3:30"`
	MapFloat32Int16   map[float32]int16   `default:"1:11,2:22,3:33"`
	MapFloat32Int32   map[float32]int32   `default:"1:111,2:222,3:333"`
	MapFloat32Int64   map[float32]int64   `default:"1:1111,2:2222,3:3333"`
	MapFloat32Uint    map[float32]uint    `default:"1:1000,2:2000,3:3000"`
	MapFloat32Uint8   map[float32]uint8   `default:"1:10,2:20,3:30"`
	MapFloat32Uint16  map[float32]uint16  `default:"1:10000,2:20000,3:30000"`
	MapFloat32Uint32  map[float32]uint32  `default:"1:100000,2:200000,3:300000"`
	MapFloat32Uint64  map[float32]uint64  `default:"1:1000000,2:2000000,3:3000000"`
	MapFloat32Float32 map[float32]float32 `default:"1:1.1,2:2.2,3:3.3"`
	MapFloat32Float64 map[float32]float64 `default:"1:111.1,2:222.2,3:333.3"`
	MapFloat32Bool    map[float32]bool    `default:"1:true,2:false,3:true"`
	MapFloat32String  map[float32]string  `default:"1:foo,2:bar"`
	MapFloat64Int     map[float64]int     `default:"1:100,2:200,3:300"`
	MapFloat64Int8    map[float64]int8    `default:"1:10,2:20,3:30"`
	MapFloat64Int16   map[float64]int16   `default:"1:11,2:22,3:33"`
	MapFloat64Int32   map[float64]int32   `default:"1:111,2:222,3:333"`
	MapFloat64Int64   map[float64]int64   `default:"1:1111,2:2222,3:3333"`
	MapFloat64Uint    map[float64]uint    `default:"1:1000,2:2000,3:3000"`
	MapFloat64Uint8   map[float64]uint8   `default:"1:10,2:20,3:30"`
	MapFloat64Uint16  map[float64]uint16  `default:"1:10000,2:20000,3:30000"`
	MapFloat64Uint32  map[float64]uint32  `default:"1:100000,2:200000,3:300000"`
	MapFloat64Uint64  map[float64]uint64  `default:"1:1000000,2:2000000,3:3000000"`
	MapFloat64Float32 map[float64]float32 `default:"1:1.1,2:2.2,3:3.3"`
	MapFloat64Float64 map[float64]float64 `default:"1:111.1,2:222.2,3:333.3"`
	MapFloat64Bool    map[float64]bool    `default:"1:true,2:false,3:true"`
	MapFloat64String  map[float64]string  `default:"1:foo,2:bar"`
	MapBoolInt        map[bool]int        `default:"true:100,false:200"`
	MapBoolInt8       map[bool]int8       `default:"true:10,false:20"`
	MapBoolInt16      map[bool]int16      `default:"true:11,false:22"`
	MapBoolInt32      map[bool]int32      `default:"true:111,false:222"`
	MapBoolInt64      map[bool]int64      `default:"true:1111,false:2222"`
	MapBoolUint       map[bool]uint       `default:"true:1000,false:2000"`
	MapBoolUint8      map[bool]uint8      `default:"true:10,false:20"`
	MapBoolUint16     map[bool]uint16     `default:"true:10000,false:20000"`
	MapBoolUint32     map[bool]uint32     `default:"true:100000,false:200000"`
	MapBoolUint64     map[bool]uint64     `default:"true:1000000,false:2000000"`
	MapBoolFloat32    map[bool]float32    `default:"true:1.1,false:2.2"`
	MapBoolFloat64    map[bool]float64    `default:"true:111.1,false:222.2"`
	MapBoolBool       map[bool]bool       `default:"true:true,false:false"`
	MapBoolString     map[bool]string     `default:"true:foo,false:bar"`
	NestedStruct      nestedStruct
}

type nestedStruct struct {
	IntNoDefault       int
	IntNoDefaultDotEnv int
	LogLevel           yalogger.Level `default:"info"`
}

var expected = testStruct{
	String:            "Ya_Code",
	Int:               42,
	Int8:              84,
	Int16:             168,
	Int32:             336,
	Int64:             672,
	Uint:              84,
	Uint8:             168,
	Uint16:            336,
	Uint32:            672,
	Uint64:            1344,
	Float:             3.14,
	Float32:           1.618,
	Float64:           2.718,
	Bool:              true,
	Bytes:             []byte{1, 2, 3},
	IntSlice:          []int{1, 2, 3},
	Int8Slice:         []int8{4, 5, 6},
	Int16Slice:        []int16{7, 8, 9},
	Int32Slice:        []int32{10, 11, 12},
	Int64Slice:        []int64{13, 14, 15},
	UintSlice:         []uint{16, 17, 18},
	Uint8Slice:        []uint8{19, 20, 21},
	Uint16Slice:       []uint16{22, 23, 24},
	Uint32Slice:       []uint32{25, 26, 27},
	Uint64Slice:       []uint64{28, 29, 30},
	FloatSlice:        []float64{31.1, 32.2, 33.3},
	Float32Slice:      []float32{34.4, 35.5, 36.6},
	Float64Slice:      []float64{37.7, 38.8, 39.9},
	BoolSlice:         []bool{true, false, true},
	StringSlice:       []string{"Ya_Code", "Skalse", "Oleksandr", "Vasab1tch", "Olderestin"},
	MapStringString:   map[string]string{"yarozpach": "OctaviaZilber"},
	MapStringInt:      map[string]int{"yashluha": 1, "anzhelchikk": 2},
	MapStringInt8:     map[string]int8{"foo": 1, "bar": 2},
	MapStringInt16:    map[string]int16{"foo": 1, "bar": 2},
	MapStringInt32:    map[string]int32{"foo": 1, "bar": 2},
	MapStringInt64:    map[string]int64{"foo": 1, "bar": 2},
	MapStringUint:     map[string]uint{"foo": 1, "bar": 2},
	MapStringUint8:    map[string]uint8{"foo": 1, "bar": 2},
	MapStringUint16:   map[string]uint16{"foo": 1, "bar": 2},
	MapStringUint32:   map[string]uint32{"foo": 1, "bar": 2},
	MapStringUint64:   map[string]uint64{"foo": 1, "bar": 2},
	MapStringFloat32:  map[string]float32{"foo": 1.1, "bar": 2.2},
	MapStringFloat64:  map[string]float64{"foo": 1.1, "bar": 2.2},
	MapStringBool:     map[string]bool{"foo": true, "bar": false},
	MapIntInt:         map[int]int{-1: 100, 2: 200, 3: 300},
	MapIntInt8:        map[int]int8{-1: 10, 2: 20, 3: 30},
	MapIntInt16:       map[int]int16{-1: 11, 2: 22, 3: 33},
	MapIntInt32:       map[int]int32{-1: 111, 2: 222, 3: 333},
	MapIntInt64:       map[int]int64{-1: 1111, 2: 2222, 3: 3333},
	MapIntUint:        map[int]uint{-1: 1000, 2: 2000, 3: 3000},
	MapIntUint8:       map[int]uint8{-1: 10, 2: 20, 3: 30},
	MapIntUint16:      map[int]uint16{-1: 10000, 2: 20000, 3: 30000},
	MapIntUint32:      map[int]uint32{-1: 100000, 2: 200000, 3: 300000},
	MapIntUint64:      map[int]uint64{-1: 1000000, 2: 2000000, 3: 3000000},
	MapIntFloat32:     map[int]float32{-1: 1.1, 2: 2.2, 3: 3.3},
	MapIntFloat64:     map[int]float64{-1: 111.1, 2: 222.2, 3: 333.3},
	MapIntBool:        map[int]bool{-1: true, 2: false, 3: true},
	MapIntString:      map[int]string{-1: "foo", 2: "bar"},
	MapInt8Int:        map[int8]int{-1: 100, 2: 200, 3: 300},
	MapInt8Int8:       map[int8]int8{-1: 10, 2: 20, 3: 30},
	MapInt8Int16:      map[int8]int16{-1: 11, 2: 22, 3: 33},
	MapInt8Int32:      map[int8]int32{-1: 111, 2: 222, 3: 333},
	MapInt8Int64:      map[int8]int64{-1: 1111, 2: 2222, 3: 3333},
	MapInt8Uint:       map[int8]uint{-1: 1000, 2: 2000, 3: 3000},
	MapInt8Uint8:      map[int8]uint8{-1: 10, 2: 20, 3: 30},
	MapInt8Uint16:     map[int8]uint16{-1: 10000, 2: 20000, 3: 30000},
	MapInt8Uint32:     map[int8]uint32{-1: 100000, 2: 200000, 3: 300000},
	MapInt8Uint64:     map[int8]uint64{-1: 1000000, 2: 2000000, 3: 3000000},
	MapInt8Float32:    map[int8]float32{-1: 1.1, 2: 2.2, 3: 3.3},
	MapInt8Float64:    map[int8]float64{-1: 111.1, 2: 222.2, 3: 333.3},
	MapInt8Bool:       map[int8]bool{-1: true, 2: false, 3: true},
	MapInt8String:     map[int8]string{-1: "foo", 2: "bar"},
	MapInt16Int:       map[int16]int{-1: 100, 2: 200, 3: 300},
	MapInt16Int8:      map[int16]int8{-1: 10, 2: 20, 3: 30},
	MapInt16Int16:     map[int16]int16{-1: 11, 2: 22, 3: 33},
	MapInt16Int32:     map[int16]int32{-1: 111, 2: 222, 3: 333},
	MapInt16Int64:     map[int16]int64{-1: 1111, 2: 2222, 3: 3333},
	MapInt16Uint:      map[int16]uint{-1: 1000, 2: 2000, 3: 3000},
	MapInt16Uint8:     map[int16]uint8{-1: 10, 2: 20, 3: 30},
	MapInt16Uint16:    map[int16]uint16{-1: 10000, 2: 20000, 3: 30000},
	MapInt16Uint32:    map[int16]uint32{-1: 100000, 2: 200000, 3: 300000},
	MapInt16Uint64:    map[int16]uint64{-1: 1000000, 2: 2000000, 3: 3000000},
	MapInt16Float32:   map[int16]float32{-1: 1.1, 2: 2.2, 3: 3.3},
	MapInt16Float64:   map[int16]float64{-1: 111.1, 2: 222.2, 3: 333.3},
	MapInt16Bool:      map[int16]bool{-1: true, 2: false, 3: true},
	MapInt16String:    map[int16]string{-1: "foo", 2: "bar"},
	MapInt32Int:       map[int32]int{-1: 100, 2: 200, 3: 300},
	MapInt32Int8:      map[int32]int8{-1: 10, 2: 20, 3: 30},
	MapInt32Int16:     map[int32]int16{-1: 11, 2: 22, 3: 33},
	MapInt32Int32:     map[int32]int32{-1: 111, 2: 222, 3: 333},
	MapInt32Int64:     map[int32]int64{-1: 1111, 2: 2222, 3: 3333},
	MapInt32Uint:      map[int32]uint{-1: 1000, 2: 2000, 3: 3000},
	MapInt32Uint8:     map[int32]uint8{-1: 10, 2: 20, 3: 30},
	MapInt32Uint16:    map[int32]uint16{-1: 10000, 2: 20000, 3: 30000},
	MapInt32Uint32:    map[int32]uint32{-1: 100000, 2: 200000, 3: 300000},
	MapInt32Uint64:    map[int32]uint64{-1: 1000000, 2: 2000000, 3: 3000000},
	MapInt32Float32:   map[int32]float32{-1: 1.1, 2: 2.2, 3: 3.3},
	MapInt32Float64:   map[int32]float64{-1: 111.1, 2: 222.2, 3: 333.3},
	MapInt32Bool:      map[int32]bool{-1: true, 2: false, 3: true},
	MapInt32String:    map[int32]string{-1: "foo", 2: "bar"},
	MapInt64Int:       map[int64]int{-1: 100, 2: 200, 3: 300},
	MapInt64Int8:      map[int64]int8{-1: 10, 2: 20, 3: 30},
	MapInt64Int16:     map[int64]int16{-1: 11, 2: 22, 3: 33},
	MapInt64Int32:     map[int64]int32{-1: 111, 2: 222, 3: 333},
	MapInt64Int64:     map[int64]int64{-1: 1111, 2: 2222, 3: 3333},
	MapInt64Uint:      map[int64]uint{-1: 1000, 2: 2000, 3: 3000},
	MapInt64Uint8:     map[int64]uint8{-1: 10, 2: 20, 3: 30},
	MapInt64Uint16:    map[int64]uint16{-1: 10000, 2: 20000, 3: 30000},
	MapInt64Uint32:    map[int64]uint32{-1: 100000, 2: 200000, 3: 300000},
	MapInt64Uint64:    map[int64]uint64{-1: 1000000, 2: 2000000, 3: 3000000},
	MapInt64Float32:   map[int64]float32{-1: 1.1, 2: 2.2, 3: 3.3},
	MapInt64Float64:   map[int64]float64{-1: 111.1, 2: 222.2, 3: 333.3},
	MapInt64Bool:      map[int64]bool{-1: true, 2: false, 3: true},
	MapInt64String:    map[int64]string{-1: "foo", 2: "bar"},
	MapUintInt:        map[uint]int{1: 100, 2: 200, 3: 300},
	MapUintInt8:       map[uint]int8{1: 10, 2: 20, 3: 30},
	MapUintInt16:      map[uint]int16{1: 11, 2: 22, 3: 33},
	MapUintInt32:      map[uint]int32{1: 111, 2: 222, 3: 333},
	MapUintInt64:      map[uint]int64{1: 1111, 2: 2222, 3: 3333},
	MapUintUint:       map[uint]uint{1: 1000, 2: 2000, 3: 3000},
	MapUintUint8:      map[uint]uint8{1: 10, 2: 20, 3: 30},
	MapUintUint16:     map[uint]uint16{1: 10000, 2: 20000, 3: 30000},
	MapUintUint32:     map[uint]uint32{1: 100000, 2: 200000, 3: 300000},
	MapUintUint64:     map[uint]uint64{1: 1000000, 2: 2000000, 3: 3000000},
	MapUintFloat32:    map[uint]float32{1: 1.1, 2: 2.2, 3: 3.3},
	MapUintFloat64:    map[uint]float64{1: 111.1, 2: 222.2, 3: 333.3},
	MapUintBool:       map[uint]bool{1: true, 2: false, 3: true},
	MapUintString:     map[uint]string{1: "foo", 2: "bar"},
	MapUint8Int:       map[uint8]int{1: 100, 2: 200, 3: 300},
	MapUint8Int8:      map[uint8]int8{1: 10, 2: 20, 3: 30},
	MapUint8Int16:     map[uint8]int16{1: 11, 2: 22, 3: 33},
	MapUint8Int32:     map[uint8]int32{1: 111, 2: 222, 3: 333},
	MapUint8Int64:     map[uint8]int64{1: 1111, 2: 2222, 3: 3333},
	MapUint8Uint:      map[uint8]uint{1: 1000, 2: 2000, 3: 3000},
	MapUint8Uint8:     map[uint8]uint8{1: 10, 2: 20, 3: 30},
	MapUint8Uint16:    map[uint8]uint16{1: 10000, 2: 20000, 3: 30000},
	MapUint8Uint32:    map[uint8]uint32{1: 100000, 2: 200000, 3: 300000},
	MapUint8Uint64:    map[uint8]uint64{1: 1000000, 2: 2000000, 3: 3000000},
	MapUint8Float32:   map[uint8]float32{1: 1.1, 2: 2.2, 3: 3.3},
	MapUint8Float64:   map[uint8]float64{1: 111.1, 2: 222.2, 3: 333.3},
	MapUint8Bool:      map[uint8]bool{1: true, 2: false, 3: true},
	MapUint8String:    map[uint8]string{1: "foo", 2: "bar"},
	MapUint16Int:      map[uint16]int{1: 100, 2: 200, 3: 300},
	MapUint16Int8:     map[uint16]int8{1: 10, 2: 20, 3: 30},
	MapUint16Int16:    map[uint16]int16{1: 11, 2: 22, 3: 33},
	MapUint16Int32:    map[uint16]int32{1: 111, 2: 222, 3: 333},
	MapUint16Int64:    map[uint16]int64{1: 1111, 2: 2222, 3: 3333},
	MapUint16Uint:     map[uint16]uint{1: 1000, 2: 2000, 3: 3000},
	MapUint16Uint8:    map[uint16]uint8{1: 10, 2: 20, 3: 30},
	MapUint16Uint16:   map[uint16]uint16{1: 10000, 2: 20000, 3: 30000},
	MapUint16Uint32:   map[uint16]uint32{1: 100000, 2: 200000, 3: 300000},
	MapUint16Uint64:   map[uint16]uint64{1: 1000000, 2: 2000000, 3: 3000000},
	MapUint16Float32:  map[uint16]float32{1: 1.1, 2: 2.2, 3: 3.3},
	MapUint16Float64:  map[uint16]float64{1: 111.1, 2: 222.2, 3: 333.3},
	MapUint16Bool:     map[uint16]bool{1: true, 2: false, 3: true},
	MapUint16String:   map[uint16]string{1: "foo", 2: "bar"},
	MapUint32Int:      map[uint32]int{1: 100, 2: 200, 3: 300},
	MapUint32Int8:     map[uint32]int8{1: 10, 2: 20, 3: 30},
	MapUint32Int16:    map[uint32]int16{1: 11, 2: 22, 3: 33},
	MapUint32Int32:    map[uint32]int32{1: 111, 2: 222, 3: 333},
	MapUint32Int64:    map[uint32]int64{1: 1111, 2: 2222, 3: 3333},
	MapUint32Uint:     map[uint32]uint{1: 1000, 2: 2000, 3: 3000},
	MapUint32Uint8:    map[uint32]uint8{1: 10, 2: 20, 3: 30},
	MapUint32Uint16:   map[uint32]uint16{1: 10000, 2: 20000, 3: 30000},
	MapUint32Uint32:   map[uint32]uint32{1: 100000, 2: 200000, 3: 300000},
	MapUint32Uint64:   map[uint32]uint64{1: 1000000, 2: 2000000, 3: 3000000},
	MapUint32Float32:  map[uint32]float32{1: 1.1, 2: 2.2, 3: 3.3},
	MapUint32Float64:  map[uint32]float64{1: 111.1, 2: 222.2, 3: 333.3},
	MapUint32Bool:     map[uint32]bool{1: true, 2: false, 3: true},
	MapUint32String:   map[uint32]string{1: "foo", 2: "bar"},
	MapUint64Int:      map[uint64]int{1: 100, 2: 200, 3: 300},
	MapUint64Int8:     map[uint64]int8{1: 10, 2: 20, 3: 30},
	MapUint64Int16:    map[uint64]int16{1: 11, 2: 22, 3: 33},
	MapUint64Int32:    map[uint64]int32{1: 111, 2: 222, 3: 333},
	MapUint64Int64:    map[uint64]int64{1: 1111, 2: 2222, 3: 3333},
	MapUint64Uint:     map[uint64]uint{1: 1000, 2: 2000, 3: 3000},
	MapUint64Uint8:    map[uint64]uint8{1: 10, 2: 20, 3: 30},
	MapUint64Uint16:   map[uint64]uint16{1: 10000, 2: 20000, 3: 30000},
	MapUint64Uint32:   map[uint64]uint32{1: 100000, 2: 200000, 3: 300000},
	MapUint64Uint64:   map[uint64]uint64{1: 1000000, 2: 2000000, 3: 3000000},
	MapUint64Float32:  map[uint64]float32{1: 1.1, 2: 2.2, 3: 3.3},
	MapUint64Float64:  map[uint64]float64{1: 111.1, 2: 222.2, 3: 333.3},
	MapUint64Bool:     map[uint64]bool{1: true, 2: false, 3: true},
	MapUint64String:   map[uint64]string{1: "foo", 2: "bar"},
	MapFloat32Int:     map[float32]int{1: 100, 2: 200, 3: 300},
	MapFloat32Int8:    map[float32]int8{1: 10, 2: 20, 3: 30},
	MapFloat32Int16:   map[float32]int16{1: 11, 2: 22, 3: 33},
	MapFloat32Int32:   map[float32]int32{1: 111, 2: 222, 3: 333},
	MapFloat32Int64:   map[float32]int64{1: 1111, 2: 2222, 3: 3333},
	MapFloat32Uint:    map[float32]uint{1: 1000, 2: 2000, 3: 3000},
	MapFloat32Uint8:   map[float32]uint8{1: 10, 2: 20, 3: 30},
	MapFloat32Uint16:  map[float32]uint16{1: 10000, 2: 20000, 3: 30000},
	MapFloat32Uint32:  map[float32]uint32{1: 100000, 2: 200000, 3: 300000},
	MapFloat32Uint64:  map[float32]uint64{1: 1000000, 2: 2000000, 3: 3000000},
	MapFloat32Float32: map[float32]float32{1: 1.1, 2: 2.2, 3: 3.3},
	MapFloat32Float64: map[float32]float64{1: 111.1, 2: 222.2, 3: 333.3},
	MapFloat32Bool:    map[float32]bool{1: true, 2: false, 3: true},
	MapFloat32String:  map[float32]string{1: "foo", 2: "bar"},
	MapFloat64Int:     map[float64]int{1: 100, 2: 200, 3: 300},
	MapFloat64Int8:    map[float64]int8{1: 10, 2: 20, 3: 30},
	MapFloat64Int16:   map[float64]int16{1: 11, 2: 22, 3: 33},
	MapFloat64Int32:   map[float64]int32{1: 111, 2: 222, 3: 333},
	MapFloat64Int64:   map[float64]int64{1: 1111, 2: 2222, 3: 3333},
	MapFloat64Uint:    map[float64]uint{1: 1000, 2: 2000, 3: 3000},
	MapFloat64Uint8:   map[float64]uint8{1: 10, 2: 20, 3: 30},
	MapFloat64Uint16:  map[float64]uint16{1: 10000, 2: 20000, 3: 30000},
	MapFloat64Uint32:  map[float64]uint32{1: 100000, 2: 200000, 3: 300000},
	MapFloat64Uint64:  map[float64]uint64{1: 1000000, 2: 2000000, 3: 3000000},
	MapFloat64Float32: map[float64]float32{1: 1.1, 2: 2.2, 3: 3.3},
	MapFloat64Float64: map[float64]float64{1: 111.1, 2: 222.2, 3: 333.3},
	MapFloat64Bool:    map[float64]bool{1: true, 2: false, 3: true},
	MapFloat64String:  map[float64]string{1: "foo", 2: "bar"},
	MapBoolInt:        map[bool]int{true: 100, false: 200},
	MapBoolInt8:       map[bool]int8{true: 10, false: 20},
	MapBoolInt16:      map[bool]int16{true: 11, false: 22},
	MapBoolInt32:      map[bool]int32{true: 111, false: 222},
	MapBoolInt64:      map[bool]int64{true: 1111, false: 2222},
	MapBoolUint:       map[bool]uint{true: 1000, false: 2000},
	MapBoolUint8:      map[bool]uint8{true: 10, false: 20},
	MapBoolUint16:     map[bool]uint16{true: 10000, false: 20000},
	MapBoolUint32:     map[bool]uint32{true: 100000, false: 200000},
	MapBoolUint64:     map[bool]uint64{true: 1000000, false: 2000000},
	MapBoolFloat32:    map[bool]float32{true: 1.1, false: 2.2},
	MapBoolFloat64:    map[bool]float64{true: 111.1, false: 222.2},
	MapBoolBool:       map[bool]bool{true: true, false: false},
	MapBoolString:     map[bool]string{true: "foo", false: "bar"},
	NestedStruct: nestedStruct{
		IntNoDefault:       100,
		IntNoDefaultDotEnv: 200,
		LogLevel:           yalogger.InfoLevel,
	},
}

func TestConfigLoader(t *testing.T) {
	t.Setenv("NESTED_STRUCT_INT_NO_DEFAULT", "100")

	file, err := os.Create(config.DotEnvFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = file.WriteString("NESTED_STRUCT_INT_NO_DEFAULT_DOT_ENV=200\n")
	if err != nil {
		t.Fatal(err)
	}

	file.Close()

	var configInstance testStruct

	err = config.LoadConfigStructFromEnvHandlingError(&configInstance, nil)

	os.Remove(config.DotEnvFile)

	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(configInstance, expected) {
		t.Errorf(
			"Expected: %+v, got: %+v, diff: %s",
			expected,
			configInstance,
			cmp.Diff(expected, configInstance),
		)
	}
}
