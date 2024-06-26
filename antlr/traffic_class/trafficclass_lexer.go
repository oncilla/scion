// File generated by ANTLR. DO NOT EDIT.

package traffic_class

import (
	"fmt"
	"github.com/antlr4-go/antlr/v4"
	"sync"
	"unicode"
)

// Suppress unused import error
var _ = fmt.Printf
var _ = sync.Once{}
var _ = unicode.IsLetter

type TrafficClassLexer struct {
	*antlr.BaseLexer
	channelNames []string
	modeNames    []string
	// TODO: EOF string
}

var TrafficClassLexerLexerStaticData struct {
	once                   sync.Once
	serializedATN          []int32
	ChannelNames           []string
	ModeNames              []string
	LiteralNames           []string
	SymbolicNames          []string
	RuleNames              []string
	PredictionContextCache *antlr.PredictionContextCache
	atn                    *antlr.ATN
	decisionToDFA          []*antlr.DFA
}

func trafficclasslexerLexerInit() {
	staticData := &TrafficClassLexerLexerStaticData
	staticData.ChannelNames = []string{
		"DEFAULT_TOKEN_CHANNEL", "HIDDEN",
	}
	staticData.ModeNames = []string{
		"DEFAULT_MODE",
	}
	staticData.LiteralNames = []string{
		"", "'='", "'=0x'", "'-'", "'cls='", "'('", "','", "')'", "'true'",
		"'false'",
	}
	staticData.SymbolicNames = []string{
		"", "", "", "", "", "", "", "", "", "", "WHITESPACE", "DIGITS", "HEX_DIGITS",
		"NET", "ANY", "ALL", "NOT", "BOOL", "SRC", "DST", "DSCP", "TOS", "PROTOCOL",
		"SRCPORT", "DSTPORT", "STRING",
	}
	staticData.RuleNames = []string{
		"T__0", "T__1", "T__2", "T__3", "T__4", "T__5", "T__6", "T__7", "T__8",
		"WHITESPACE", "DIGITS", "HEX_DIGITS", "NET", "ANY", "ALL", "NOT", "BOOL",
		"SRC", "DST", "DSCP", "TOS", "PROTOCOL", "SRCPORT", "DSTPORT", "STRING",
	}
	staticData.PredictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 0, 25, 236, 6, -1, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2,
		4, 7, 4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 2, 8, 7, 8, 2, 9, 7, 9, 2,
		10, 7, 10, 2, 11, 7, 11, 2, 12, 7, 12, 2, 13, 7, 13, 2, 14, 7, 14, 2, 15,
		7, 15, 2, 16, 7, 16, 2, 17, 7, 17, 2, 18, 7, 18, 2, 19, 7, 19, 2, 20, 7,
		20, 2, 21, 7, 21, 2, 22, 7, 22, 2, 23, 7, 23, 2, 24, 7, 24, 1, 0, 1, 0,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 1, 2, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 4,
		1, 4, 1, 5, 1, 5, 1, 6, 1, 6, 1, 7, 1, 7, 1, 7, 1, 7, 1, 7, 1, 8, 1, 8,
		1, 8, 1, 8, 1, 8, 1, 8, 1, 9, 4, 9, 83, 8, 9, 11, 9, 12, 9, 84, 1, 9, 1,
		9, 1, 10, 1, 10, 1, 10, 5, 10, 92, 8, 10, 10, 10, 12, 10, 95, 9, 10, 3,
		10, 97, 8, 10, 1, 11, 4, 11, 100, 8, 11, 11, 11, 12, 11, 101, 1, 12, 1,
		12, 1, 12, 1, 12, 1, 12, 1, 12, 1, 12, 1, 12, 1, 12, 1, 12, 1, 13, 1, 13,
		1, 13, 1, 13, 1, 13, 1, 13, 3, 13, 120, 8, 13, 1, 14, 1, 14, 1, 14, 1,
		14, 1, 14, 1, 14, 3, 14, 128, 8, 14, 1, 15, 1, 15, 1, 15, 1, 15, 1, 15,
		1, 15, 3, 15, 136, 8, 15, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1,
		16, 1, 16, 3, 16, 146, 8, 16, 1, 17, 1, 17, 1, 17, 1, 17, 1, 17, 1, 17,
		3, 17, 154, 8, 17, 1, 18, 1, 18, 1, 18, 1, 18, 1, 18, 1, 18, 3, 18, 162,
		8, 18, 1, 19, 1, 19, 1, 19, 1, 19, 1, 19, 1, 19, 1, 19, 1, 19, 3, 19, 172,
		8, 19, 1, 20, 1, 20, 1, 20, 1, 20, 1, 20, 1, 20, 3, 20, 180, 8, 20, 1,
		21, 1, 21, 1, 21, 1, 21, 1, 21, 1, 21, 1, 21, 1, 21, 1, 21, 1, 21, 1, 21,
		1, 21, 1, 21, 1, 21, 1, 21, 1, 21, 3, 21, 198, 8, 21, 1, 22, 1, 22, 1,
		22, 1, 22, 1, 22, 1, 22, 1, 22, 1, 22, 1, 22, 1, 22, 1, 22, 1, 22, 1, 22,
		1, 22, 3, 22, 214, 8, 22, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1,
		23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 3, 23, 230, 8, 23,
		1, 24, 4, 24, 233, 8, 24, 11, 24, 12, 24, 234, 0, 0, 25, 1, 1, 3, 2, 5,
		3, 7, 4, 9, 5, 11, 6, 13, 7, 15, 8, 17, 9, 19, 10, 21, 11, 23, 12, 25,
		13, 27, 14, 29, 15, 31, 16, 33, 17, 35, 18, 37, 19, 39, 20, 41, 21, 43,
		22, 45, 23, 47, 24, 49, 25, 1, 0, 5, 3, 0, 9, 10, 13, 13, 32, 32, 1, 0,
		49, 57, 1, 0, 48, 57, 3, 0, 48, 57, 65, 70, 97, 102, 2, 0, 65, 90, 97,
		122, 251, 0, 1, 1, 0, 0, 0, 0, 3, 1, 0, 0, 0, 0, 5, 1, 0, 0, 0, 0, 7, 1,
		0, 0, 0, 0, 9, 1, 0, 0, 0, 0, 11, 1, 0, 0, 0, 0, 13, 1, 0, 0, 0, 0, 15,
		1, 0, 0, 0, 0, 17, 1, 0, 0, 0, 0, 19, 1, 0, 0, 0, 0, 21, 1, 0, 0, 0, 0,
		23, 1, 0, 0, 0, 0, 25, 1, 0, 0, 0, 0, 27, 1, 0, 0, 0, 0, 29, 1, 0, 0, 0,
		0, 31, 1, 0, 0, 0, 0, 33, 1, 0, 0, 0, 0, 35, 1, 0, 0, 0, 0, 37, 1, 0, 0,
		0, 0, 39, 1, 0, 0, 0, 0, 41, 1, 0, 0, 0, 0, 43, 1, 0, 0, 0, 0, 45, 1, 0,
		0, 0, 0, 47, 1, 0, 0, 0, 0, 49, 1, 0, 0, 0, 1, 51, 1, 0, 0, 0, 3, 53, 1,
		0, 0, 0, 5, 57, 1, 0, 0, 0, 7, 59, 1, 0, 0, 0, 9, 64, 1, 0, 0, 0, 11, 66,
		1, 0, 0, 0, 13, 68, 1, 0, 0, 0, 15, 70, 1, 0, 0, 0, 17, 75, 1, 0, 0, 0,
		19, 82, 1, 0, 0, 0, 21, 96, 1, 0, 0, 0, 23, 99, 1, 0, 0, 0, 25, 103, 1,
		0, 0, 0, 27, 119, 1, 0, 0, 0, 29, 127, 1, 0, 0, 0, 31, 135, 1, 0, 0, 0,
		33, 145, 1, 0, 0, 0, 35, 153, 1, 0, 0, 0, 37, 161, 1, 0, 0, 0, 39, 171,
		1, 0, 0, 0, 41, 179, 1, 0, 0, 0, 43, 197, 1, 0, 0, 0, 45, 213, 1, 0, 0,
		0, 47, 229, 1, 0, 0, 0, 49, 232, 1, 0, 0, 0, 51, 52, 5, 61, 0, 0, 52, 2,
		1, 0, 0, 0, 53, 54, 5, 61, 0, 0, 54, 55, 5, 48, 0, 0, 55, 56, 5, 120, 0,
		0, 56, 4, 1, 0, 0, 0, 57, 58, 5, 45, 0, 0, 58, 6, 1, 0, 0, 0, 59, 60, 5,
		99, 0, 0, 60, 61, 5, 108, 0, 0, 61, 62, 5, 115, 0, 0, 62, 63, 5, 61, 0,
		0, 63, 8, 1, 0, 0, 0, 64, 65, 5, 40, 0, 0, 65, 10, 1, 0, 0, 0, 66, 67,
		5, 44, 0, 0, 67, 12, 1, 0, 0, 0, 68, 69, 5, 41, 0, 0, 69, 14, 1, 0, 0,
		0, 70, 71, 5, 116, 0, 0, 71, 72, 5, 114, 0, 0, 72, 73, 5, 117, 0, 0, 73,
		74, 5, 101, 0, 0, 74, 16, 1, 0, 0, 0, 75, 76, 5, 102, 0, 0, 76, 77, 5,
		97, 0, 0, 77, 78, 5, 108, 0, 0, 78, 79, 5, 115, 0, 0, 79, 80, 5, 101, 0,
		0, 80, 18, 1, 0, 0, 0, 81, 83, 7, 0, 0, 0, 82, 81, 1, 0, 0, 0, 83, 84,
		1, 0, 0, 0, 84, 82, 1, 0, 0, 0, 84, 85, 1, 0, 0, 0, 85, 86, 1, 0, 0, 0,
		86, 87, 6, 9, 0, 0, 87, 20, 1, 0, 0, 0, 88, 97, 5, 48, 0, 0, 89, 93, 7,
		1, 0, 0, 90, 92, 7, 2, 0, 0, 91, 90, 1, 0, 0, 0, 92, 95, 1, 0, 0, 0, 93,
		91, 1, 0, 0, 0, 93, 94, 1, 0, 0, 0, 94, 97, 1, 0, 0, 0, 95, 93, 1, 0, 0,
		0, 96, 88, 1, 0, 0, 0, 96, 89, 1, 0, 0, 0, 97, 22, 1, 0, 0, 0, 98, 100,
		7, 3, 0, 0, 99, 98, 1, 0, 0, 0, 100, 101, 1, 0, 0, 0, 101, 99, 1, 0, 0,
		0, 101, 102, 1, 0, 0, 0, 102, 24, 1, 0, 0, 0, 103, 104, 3, 21, 10, 0, 104,
		105, 5, 46, 0, 0, 105, 106, 3, 21, 10, 0, 106, 107, 5, 46, 0, 0, 107, 108,
		3, 21, 10, 0, 108, 109, 5, 46, 0, 0, 109, 110, 3, 21, 10, 0, 110, 111,
		5, 47, 0, 0, 111, 112, 3, 21, 10, 0, 112, 26, 1, 0, 0, 0, 113, 114, 5,
		65, 0, 0, 114, 115, 5, 78, 0, 0, 115, 120, 5, 89, 0, 0, 116, 117, 5, 97,
		0, 0, 117, 118, 5, 110, 0, 0, 118, 120, 5, 121, 0, 0, 119, 113, 1, 0, 0,
		0, 119, 116, 1, 0, 0, 0, 120, 28, 1, 0, 0, 0, 121, 122, 5, 65, 0, 0, 122,
		123, 5, 76, 0, 0, 123, 128, 5, 76, 0, 0, 124, 125, 5, 97, 0, 0, 125, 126,
		5, 108, 0, 0, 126, 128, 5, 108, 0, 0, 127, 121, 1, 0, 0, 0, 127, 124, 1,
		0, 0, 0, 128, 30, 1, 0, 0, 0, 129, 130, 5, 78, 0, 0, 130, 131, 5, 79, 0,
		0, 131, 136, 5, 84, 0, 0, 132, 133, 5, 110, 0, 0, 133, 134, 5, 111, 0,
		0, 134, 136, 5, 116, 0, 0, 135, 129, 1, 0, 0, 0, 135, 132, 1, 0, 0, 0,
		136, 32, 1, 0, 0, 0, 137, 138, 5, 66, 0, 0, 138, 139, 5, 79, 0, 0, 139,
		140, 5, 79, 0, 0, 140, 146, 5, 76, 0, 0, 141, 142, 5, 98, 0, 0, 142, 143,
		5, 111, 0, 0, 143, 144, 5, 111, 0, 0, 144, 146, 5, 108, 0, 0, 145, 137,
		1, 0, 0, 0, 145, 141, 1, 0, 0, 0, 146, 34, 1, 0, 0, 0, 147, 148, 5, 83,
		0, 0, 148, 149, 5, 82, 0, 0, 149, 154, 5, 67, 0, 0, 150, 151, 5, 115, 0,
		0, 151, 152, 5, 114, 0, 0, 152, 154, 5, 99, 0, 0, 153, 147, 1, 0, 0, 0,
		153, 150, 1, 0, 0, 0, 154, 36, 1, 0, 0, 0, 155, 156, 5, 68, 0, 0, 156,
		157, 5, 83, 0, 0, 157, 162, 5, 84, 0, 0, 158, 159, 5, 100, 0, 0, 159, 160,
		5, 115, 0, 0, 160, 162, 5, 116, 0, 0, 161, 155, 1, 0, 0, 0, 161, 158, 1,
		0, 0, 0, 162, 38, 1, 0, 0, 0, 163, 164, 5, 68, 0, 0, 164, 165, 5, 83, 0,
		0, 165, 166, 5, 67, 0, 0, 166, 172, 5, 80, 0, 0, 167, 168, 5, 100, 0, 0,
		168, 169, 5, 115, 0, 0, 169, 170, 5, 99, 0, 0, 170, 172, 5, 112, 0, 0,
		171, 163, 1, 0, 0, 0, 171, 167, 1, 0, 0, 0, 172, 40, 1, 0, 0, 0, 173, 174,
		5, 84, 0, 0, 174, 175, 5, 79, 0, 0, 175, 180, 5, 83, 0, 0, 176, 177, 5,
		116, 0, 0, 177, 178, 5, 111, 0, 0, 178, 180, 5, 115, 0, 0, 179, 173, 1,
		0, 0, 0, 179, 176, 1, 0, 0, 0, 180, 42, 1, 0, 0, 0, 181, 182, 5, 80, 0,
		0, 182, 183, 5, 82, 0, 0, 183, 184, 5, 79, 0, 0, 184, 185, 5, 84, 0, 0,
		185, 186, 5, 79, 0, 0, 186, 187, 5, 67, 0, 0, 187, 188, 5, 79, 0, 0, 188,
		198, 5, 76, 0, 0, 189, 190, 5, 112, 0, 0, 190, 191, 5, 114, 0, 0, 191,
		192, 5, 111, 0, 0, 192, 193, 5, 116, 0, 0, 193, 194, 5, 111, 0, 0, 194,
		195, 5, 99, 0, 0, 195, 196, 5, 111, 0, 0, 196, 198, 5, 108, 0, 0, 197,
		181, 1, 0, 0, 0, 197, 189, 1, 0, 0, 0, 198, 44, 1, 0, 0, 0, 199, 200, 5,
		83, 0, 0, 200, 201, 5, 82, 0, 0, 201, 202, 5, 67, 0, 0, 202, 203, 5, 80,
		0, 0, 203, 204, 5, 79, 0, 0, 204, 205, 5, 82, 0, 0, 205, 214, 5, 84, 0,
		0, 206, 207, 5, 115, 0, 0, 207, 208, 5, 114, 0, 0, 208, 209, 5, 99, 0,
		0, 209, 210, 5, 112, 0, 0, 210, 211, 5, 111, 0, 0, 211, 212, 5, 114, 0,
		0, 212, 214, 5, 116, 0, 0, 213, 199, 1, 0, 0, 0, 213, 206, 1, 0, 0, 0,
		214, 46, 1, 0, 0, 0, 215, 216, 5, 68, 0, 0, 216, 217, 5, 83, 0, 0, 217,
		218, 5, 84, 0, 0, 218, 219, 5, 80, 0, 0, 219, 220, 5, 79, 0, 0, 220, 221,
		5, 82, 0, 0, 221, 230, 5, 84, 0, 0, 222, 223, 5, 100, 0, 0, 223, 224, 5,
		115, 0, 0, 224, 225, 5, 116, 0, 0, 225, 226, 5, 112, 0, 0, 226, 227, 5,
		111, 0, 0, 227, 228, 5, 114, 0, 0, 228, 230, 5, 116, 0, 0, 229, 215, 1,
		0, 0, 0, 229, 222, 1, 0, 0, 0, 230, 48, 1, 0, 0, 0, 231, 233, 7, 4, 0,
		0, 232, 231, 1, 0, 0, 0, 233, 234, 1, 0, 0, 0, 234, 232, 1, 0, 0, 0, 234,
		235, 1, 0, 0, 0, 235, 50, 1, 0, 0, 0, 18, 0, 84, 93, 96, 99, 101, 119,
		127, 135, 145, 153, 161, 171, 179, 197, 213, 229, 234, 1, 6, 0, 0,
	}
	deserializer := antlr.NewATNDeserializer(nil)
	staticData.atn = deserializer.Deserialize(staticData.serializedATN)
	atn := staticData.atn
	staticData.decisionToDFA = make([]*antlr.DFA, len(atn.DecisionToState))
	decisionToDFA := staticData.decisionToDFA
	for index, state := range atn.DecisionToState {
		decisionToDFA[index] = antlr.NewDFA(state, index)
	}
}

// TrafficClassLexerInit initializes any static state used to implement TrafficClassLexer. By default the
// static state used to implement the lexer is lazily initialized during the first call to
// NewTrafficClassLexer(). You can call this function if you wish to initialize the static state ahead
// of time.
func TrafficClassLexerInit() {
	staticData := &TrafficClassLexerLexerStaticData
	staticData.once.Do(trafficclasslexerLexerInit)
}

// NewTrafficClassLexer produces a new lexer instance for the optional input antlr.CharStream.
func NewTrafficClassLexer(input antlr.CharStream) *TrafficClassLexer {
	TrafficClassLexerInit()
	l := new(TrafficClassLexer)
	l.BaseLexer = antlr.NewBaseLexer(input)
	staticData := &TrafficClassLexerLexerStaticData
	l.Interpreter = antlr.NewLexerATNSimulator(l, staticData.atn, staticData.decisionToDFA, staticData.PredictionContextCache)
	l.channelNames = staticData.ChannelNames
	l.modeNames = staticData.ModeNames
	l.RuleNames = staticData.RuleNames
	l.LiteralNames = staticData.LiteralNames
	l.SymbolicNames = staticData.SymbolicNames
	l.GrammarFileName = "TrafficClass.g4"
	// TODO: l.EOF = antlr.TokenEOF

	return l
}

// TrafficClassLexer tokens.
const (
	TrafficClassLexerT__0       = 1
	TrafficClassLexerT__1       = 2
	TrafficClassLexerT__2       = 3
	TrafficClassLexerT__3       = 4
	TrafficClassLexerT__4       = 5
	TrafficClassLexerT__5       = 6
	TrafficClassLexerT__6       = 7
	TrafficClassLexerT__7       = 8
	TrafficClassLexerT__8       = 9
	TrafficClassLexerWHITESPACE = 10
	TrafficClassLexerDIGITS     = 11
	TrafficClassLexerHEX_DIGITS = 12
	TrafficClassLexerNET        = 13
	TrafficClassLexerANY        = 14
	TrafficClassLexerALL        = 15
	TrafficClassLexerNOT        = 16
	TrafficClassLexerBOOL       = 17
	TrafficClassLexerSRC        = 18
	TrafficClassLexerDST        = 19
	TrafficClassLexerDSCP       = 20
	TrafficClassLexerTOS        = 21
	TrafficClassLexerPROTOCOL   = 22
	TrafficClassLexerSRCPORT    = 23
	TrafficClassLexerDSTPORT    = 24
	TrafficClassLexerSTRING     = 25
)
