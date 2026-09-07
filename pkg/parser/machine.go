
//line machine.rl:1
// compile-command: "ragel -Z -G2 machine.rl"

package parser

import "fmt"

const maxstack = 64


//line machine.rl:101



//line machine.go:14
const expr_start int = 19
const expr_first_final int = 19
const expr_error int = 0

const expr_en_qstring int = 72
const expr_en_istring int = 75
const expr_en_expr int = 19


//line machine.rl:104

func lexData(data []byte, r *lexResult) (err error) {
	var cs, act, ts, te, top int
	var stack [maxstack]int
	p, pe := 0, len(data)
	eof := pe

//line machine.go:30
	{
	cs = expr_start
	top = 0
	ts = 0
	te = 0
	act = 0
	}

//line machine.rl:111
	_, _, _, _, _ = expr_first_final, expr_error, expr_en_qstring, expr_en_istring, expr_en_expr
	if r.file == nil {
		r.file = fileset.AddFile("(string)", -1, len(data))
	}
	if r.data == nil {
		r.data = data
	} else {
		r.data = append(r.data, data...)
	}
	nostack := func() bool {
		if top != maxstack {
			return false
		}
		err = r.Errorf("exceeds recursion limit")
		return true
	}
	tok := func(sym int) { r.tokens = append(r.tokens, lexerToken{sym: sym, pos: ts, end: te}) }
	tokcomment := func(sym int) { r.comments = append(r.comments, lexerToken{sym: sym, pos: ts, end: te}) }
	var backrefs Backrefs
	tokenter := func(sym, fin int) { backrefs.Push(len(r.tokens), fin); tok(sym); }
	tokleave := func(sym int) bool {
		tok(sym)
		if top == 0 || len(backrefs) == 0 {
			err = r.Errorf("does not close anything")
			return false
		}
		iprev, prevsym := backrefs.Pop()
		if prevsym != sym {
			err = r.Errorf(fmt.Sprintf("does not close %v", r.tokens[iprev]))
			return false
		}
		r.tokens[len(r.tokens)-1].prev = iprev
		return true
	}
	tokarg := func() bool {
		tok(int(data[ts]))
		if len(r.tokens) == 1 {
			err = r.Errorf("does not follow anything")
			return false
		}
		prev := &r.tokens[len(r.tokens)-2]
		switch prev.sym {
		case id:
			prev.sym = argID
		case '}':
			r.tokens[prev.prev].sym = argBracket
		default:
			err = r.Errorf("does not follow an argument of a function")
			return false
		}
		return true
	}
	addLines := func() {
		for i := ts; i < te; i++ {
			if data[i] == '\n' {
				r.file.AddLine(i)
			}
		}
	}


//line machine.go:99
	{
	if p == pe {
		goto _test_eof
	}
	goto _resume

_again:
	switch cs {
	case 19:
		goto st19
	case 20:
		goto st20
	case 21:
		goto st21
	case 22:
		goto st22
	case 23:
		goto st23
	case 24:
		goto st24
	case 25:
		goto st25
	case 26:
		goto st26
	case 27:
		goto st27
	case 1:
		goto st1
	case 2:
		goto st2
	case 28:
		goto st28
	case 29:
		goto st29
	case 30:
		goto st30
	case 31:
		goto st31
	case 3:
		goto st3
	case 32:
		goto st32
	case 4:
		goto st4
	case 5:
		goto st5
	case 33:
		goto st33
	case 34:
		goto st34
	case 6:
		goto st6
	case 7:
		goto st7
	case 35:
		goto st35
	case 36:
		goto st36
	case 8:
		goto st8
	case 9:
		goto st9
	case 37:
		goto st37
	case 38:
		goto st38
	case 39:
		goto st39
	case 40:
		goto st40
	case 10:
		goto st10
	case 11:
		goto st11
	case 41:
		goto st41
	case 42:
		goto st42
	case 43:
		goto st43
	case 44:
		goto st44
	case 45:
		goto st45
	case 46:
		goto st46
	case 47:
		goto st47
	case 48:
		goto st48
	case 49:
		goto st49
	case 50:
		goto st50
	case 51:
		goto st51
	case 52:
		goto st52
	case 53:
		goto st53
	case 54:
		goto st54
	case 55:
		goto st55
	case 56:
		goto st56
	case 57:
		goto st57
	case 58:
		goto st58
	case 59:
		goto st59
	case 60:
		goto st60
	case 61:
		goto st61
	case 62:
		goto st62
	case 63:
		goto st63
	case 64:
		goto st64
	case 65:
		goto st65
	case 66:
		goto st66
	case 67:
		goto st67
	case 68:
		goto st68
	case 69:
		goto st69
	case 12:
		goto st12
	case 70:
		goto st70
	case 71:
		goto st71
	case 72:
		goto st72
	case 73:
		goto st73
	case 13:
		goto st13
	case 14:
		goto st14
	case 74:
		goto st74
	case 75:
		goto st75
	case 76:
		goto st76
	case 15:
		goto st15
	case 16:
		goto st16
	case 17:
		goto st17
	case 18:
		goto st18
	case 77:
		goto st77
	case 78:
		goto st78
	case 79:
		goto st79
	case 0:
		goto st0
	}

	if p++; p == pe {
		goto _test_eof
	}
_resume:
	switch cs {
	case 19:
		goto st_case_19
	case 20:
		goto st_case_20
	case 21:
		goto st_case_21
	case 22:
		goto st_case_22
	case 23:
		goto st_case_23
	case 24:
		goto st_case_24
	case 25:
		goto st_case_25
	case 26:
		goto st_case_26
	case 27:
		goto st_case_27
	case 1:
		goto st_case_1
	case 2:
		goto st_case_2
	case 28:
		goto st_case_28
	case 29:
		goto st_case_29
	case 30:
		goto st_case_30
	case 31:
		goto st_case_31
	case 3:
		goto st_case_3
	case 32:
		goto st_case_32
	case 4:
		goto st_case_4
	case 5:
		goto st_case_5
	case 33:
		goto st_case_33
	case 34:
		goto st_case_34
	case 6:
		goto st_case_6
	case 7:
		goto st_case_7
	case 35:
		goto st_case_35
	case 36:
		goto st_case_36
	case 8:
		goto st_case_8
	case 9:
		goto st_case_9
	case 37:
		goto st_case_37
	case 38:
		goto st_case_38
	case 39:
		goto st_case_39
	case 40:
		goto st_case_40
	case 10:
		goto st_case_10
	case 11:
		goto st_case_11
	case 41:
		goto st_case_41
	case 42:
		goto st_case_42
	case 43:
		goto st_case_43
	case 44:
		goto st_case_44
	case 45:
		goto st_case_45
	case 46:
		goto st_case_46
	case 47:
		goto st_case_47
	case 48:
		goto st_case_48
	case 49:
		goto st_case_49
	case 50:
		goto st_case_50
	case 51:
		goto st_case_51
	case 52:
		goto st_case_52
	case 53:
		goto st_case_53
	case 54:
		goto st_case_54
	case 55:
		goto st_case_55
	case 56:
		goto st_case_56
	case 57:
		goto st_case_57
	case 58:
		goto st_case_58
	case 59:
		goto st_case_59
	case 60:
		goto st_case_60
	case 61:
		goto st_case_61
	case 62:
		goto st_case_62
	case 63:
		goto st_case_63
	case 64:
		goto st_case_64
	case 65:
		goto st_case_65
	case 66:
		goto st_case_66
	case 67:
		goto st_case_67
	case 68:
		goto st_case_68
	case 69:
		goto st_case_69
	case 12:
		goto st_case_12
	case 70:
		goto st_case_70
	case 71:
		goto st_case_71
	case 72:
		goto st_case_72
	case 73:
		goto st_case_73
	case 13:
		goto st_case_13
	case 14:
		goto st_case_14
	case 74:
		goto st_case_74
	case 75:
		goto st_case_75
	case 76:
		goto st_case_76
	case 15:
		goto st_case_15
	case 16:
		goto st_case_16
	case 17:
		goto st_case_17
	case 18:
		goto st_case_18
	case 77:
		goto st_case_77
	case 78:
		goto st_case_78
	case 79:
		goto st_case_79
	case 0:
		goto st_case_0
	}
	goto st_out
tr0:
//line NONE:1
	switch act {
	case 9:
	{p = (te) - 1
 tok(assert_) }
	case 10:
	{p = (te) - 1
 tok(else_) }
	case 11:
	{p = (te) - 1
 tok(if_) }
	case 12:
	{p = (te) - 1
 tok(in) }
	case 13:
	{p = (te) - 1
 tok(inherit) }
	case 14:
	{p = (te) - 1
 tok(let) }
	case 15:
	{p = (te) - 1
 tok(or_) }
	case 16:
	{p = (te) - 1
 tok(rec) }
	case 17:
	{p = (te) - 1
 tok(then) }
	case 18:
	{p = (te) - 1
 tok(with) }
	case 27:
	{p = (te) - 1
 tok(float) }
	case 28:
	{p = (te) - 1
 tok(int_) }
	case 29:
	{p = (te) - 1
 tok(id) }
	case 30:
	{p = (te) - 1
 tok(ellipsis) }
	case 39:
	{p = (te) - 1
 tok(concat) }
	case 50:
	{p = (te) - 1
 tok(int(data[ts])) }
	}
	
	goto st19
tr4:
//line machine.rl:97
p = (te) - 1
{ tok(int(data[ts])) }
	goto st19
tr6:
//line machine.rl:69
p = (te) - 1
{ tok(float) }
	goto st19
tr11:
//line machine.rl:64
te = p+1
{ tokcomment(comment); addLines() }
	goto st19
tr14:
//line machine.rl:68
te = p+1
{ tok(path) }
	goto st19
tr27:
//line machine.rl:97
te = p+1
{ tok(int(data[ts])) }
	goto st19
tr29:
//line machine.rl:62
te = p+1
{ r.file.AddLine(ts) }
	goto st19
tr31:
//line machine.rl:86
te = p+1
{ tokenter('"', '"'); { if nostack() { return }; {stack[top] = 19; top++; goto st72 }} }
	goto st19
tr36:
//line machine.rl:90
te = p+1
{ tokenter('(', ')'); { if nostack() { return }; {stack[top] = 19; top++; goto st19 }} }
	goto st19
tr37:
//line machine.rl:93
te = p+1
{ if !tokleave(int(data[ts])) { return }; {top--; cs = stack[top];goto _again } }
	goto st19
tr43:
//line machine.rl:95
te = p+1
{ if !tokarg() { return }; }
	goto st19
tr48:
//line machine.rl:91
te = p+1
{ tokenter('[', ']'); { if nostack() { return }; {stack[top] = 19; top++; goto st19 }} }
	goto st19
tr58:
//line machine.rl:92
te = p+1
{ tokenter('{', '}'); { if nostack() { return }; {stack[top] = 19; top++; goto st19 }} }
	goto st19
tr61:
//line machine.rl:61
te = p
p--

	goto st19
tr62:
//line machine.rl:97
te = p
p--
{ tok(int(data[ts])) }
	goto st19
tr63:
//line machine.rl:78
te = p+1
{ tok(neq) }
	goto st19
tr64:
//line machine.rl:63
te = p
p--
{ tokcomment(comment) }
	goto st19
tr65:
//line machine.rl:89
te = p+1
{ tokenter(interp, '}'); { if nostack() { return }; {stack[top] = 19; top++; goto st19 }} }
	goto st19
tr66:
//line machine.rl:76
te = p+1
{ tok(and) }
	goto st19
tr67:
//line machine.rl:87
te = p+1
{ tokenter(ii, ii); { if nostack() { return }; {stack[top] = 19; top++; goto st75 }}  }
	goto st19
tr69:
//line machine.rl:66
te = p
p--
{ tok(path) }
	goto st19
tr71:
//line machine.rl:74
te = p+1
{ tok(impl) }
	goto st19
tr74:
//line machine.rl:69
te = p
p--
{ tok(float) }
	goto st19
tr76:
//line machine.rl:81
te = p+1
{ tok(update) }
	goto st19
tr77:
//line machine.rl:70
te = p
p--
{ tok(int_) }
	goto st19
tr78:
//line machine.rl:79
te = p+1
{ tok(leq) }
	goto st19
tr79:
//line machine.rl:84
te = p+1
{ tok(pipe_from) }
	goto st19
tr80:
//line machine.rl:77
te = p+1
{ tok(eq) }
	goto st19
tr81:
//line machine.rl:80
te = p+1
{ tok(geq) }
	goto st19
tr83:
//line machine.rl:71
te = p
p--
{ tok(id) }
	goto st19
tr84:
//line machine.rl:65
te = p
p--
{ tok(uri) }
	goto st19
tr95:
//line machine.rl:53
te = p
p--
{ tok(in) }
	goto st19
tr112:
//line machine.rl:83
te = p+1
{ tok(pipe_into) }
	goto st19
tr113:
//line machine.rl:75
te = p+1
{ tok(or) }
	goto st19
tr115:
//line machine.rl:67
te = p
p--
{ tok(path) }
	goto st19
	st19:
//line NONE:1
ts = 0

		if p++; p == pe {
			goto _test_eof19
		}
	st_case_19:
//line NONE:1
ts = p

//line machine.go:682
		switch data[p] {
		case 9:
			goto st20
		case 10:
			goto tr29
		case 13:
			goto st20
		case 32:
			goto st20
		case 33:
			goto st21
		case 34:
			goto tr31
		case 35:
			goto st22
		case 36:
			goto st23
		case 38:
			goto st24
		case 39:
			goto st25
		case 40:
			goto tr36
		case 41:
			goto tr37
		case 43:
			goto tr38
		case 45:
			goto tr39
		case 46:
			goto tr40
		case 47:
			goto tr41
		case 58:
			goto tr43
		case 60:
			goto tr44
		case 61:
			goto st37
		case 62:
			goto st38
		case 64:
			goto tr43
		case 91:
			goto tr48
		case 93:
			goto tr37
		case 95:
			goto tr49
		case 97:
			goto tr50
		case 101:
			goto tr51
		case 105:
			goto tr52
		case 108:
			goto tr53
		case 111:
			goto tr54
		case 114:
			goto tr55
		case 116:
			goto tr56
		case 119:
			goto tr57
		case 123:
			goto tr58
		case 124:
			goto st68
		case 125:
			goto tr37
		case 126:
			goto tr60
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr42
			}
		case data[p] > 90:
			if 98 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr27
	st20:
		if p++; p == pe {
			goto _test_eof20
		}
	st_case_20:
		switch data[p] {
		case 9:
			goto st20
		case 13:
			goto st20
		case 32:
			goto st20
		}
		goto tr61
	st21:
		if p++; p == pe {
			goto _test_eof21
		}
	st_case_21:
		if data[p] == 61 {
			goto tr63
		}
		goto tr62
	st22:
		if p++; p == pe {
			goto _test_eof22
		}
	st_case_22:
		switch data[p] {
		case 10:
			goto tr64
		case 13:
			goto tr64
		}
		goto st22
	st23:
		if p++; p == pe {
			goto _test_eof23
		}
	st_case_23:
		if data[p] == 123 {
			goto tr65
		}
		goto tr62
	st24:
		if p++; p == pe {
			goto _test_eof24
		}
	st_case_24:
		if data[p] == 38 {
			goto tr66
		}
		goto tr62
	st25:
		if p++; p == pe {
			goto _test_eof25
		}
	st_case_25:
		if data[p] == 39 {
			goto tr67
		}
		goto tr62
tr38:
//line NONE:1
te = p+1

//line machine.rl:97
act = 50;
	goto st26
	st26:
		if p++; p == pe {
			goto _test_eof26
		}
	st_case_26:
//line machine.go:844
		switch data[p] {
		case 43:
			goto tr68
		case 47:
			goto st2
		case 95:
			goto st1
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto st1
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st1
			}
		default:
			goto st1
		}
		goto tr62
tr5:
//line NONE:1
te = p+1

//line machine.rl:73
act = 30;
	goto st27
tr68:
//line NONE:1
te = p+1

//line machine.rl:82
act = 39;
	goto st27
	st27:
		if p++; p == pe {
			goto _test_eof27
		}
	st_case_27:
//line machine.go:885
		switch data[p] {
		case 43:
			goto st1
		case 47:
			goto st2
		case 95:
			goto st1
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto st1
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st1
			}
		default:
			goto st1
		}
		goto tr0
	st1:
		if p++; p == pe {
			goto _test_eof1
		}
	st_case_1:
		switch data[p] {
		case 43:
			goto st1
		case 47:
			goto st2
		case 95:
			goto st1
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto st1
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st1
			}
		default:
			goto st1
		}
		goto tr0
	st2:
		if p++; p == pe {
			goto _test_eof2
		}
	st_case_2:
		switch data[p] {
		case 43:
			goto st28
		case 95:
			goto st28
		}
		switch {
		case data[p] < 48:
			if 45 <= data[p] && data[p] <= 46 {
				goto st28
			}
		case data[p] > 57:
			switch {
			case data[p] > 90:
				if 97 <= data[p] && data[p] <= 122 {
					goto st28
				}
			case data[p] >= 65:
				goto st28
			}
		default:
			goto st28
		}
		goto tr0
	st28:
		if p++; p == pe {
			goto _test_eof28
		}
	st_case_28:
		switch data[p] {
		case 43:
			goto st28
		case 47:
			goto st29
		case 95:
			goto st28
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto st28
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st28
			}
		default:
			goto st28
		}
		goto tr69
	st29:
		if p++; p == pe {
			goto _test_eof29
		}
	st_case_29:
		switch data[p] {
		case 43:
			goto st28
		case 95:
			goto st28
		}
		switch {
		case data[p] < 48:
			if 45 <= data[p] && data[p] <= 46 {
				goto st28
			}
		case data[p] > 57:
			switch {
			case data[p] > 90:
				if 97 <= data[p] && data[p] <= 122 {
					goto st28
				}
			case data[p] >= 65:
				goto st28
			}
		default:
			goto st28
		}
		goto tr69
tr39:
//line NONE:1
te = p+1

//line machine.rl:97
act = 50;
	goto st30
	st30:
		if p++; p == pe {
			goto _test_eof30
		}
	st_case_30:
//line machine.go:1029
		switch data[p] {
		case 43:
			goto st1
		case 47:
			goto st2
		case 62:
			goto tr71
		case 95:
			goto st1
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto st1
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st1
			}
		default:
			goto st1
		}
		goto tr62
tr40:
//line NONE:1
te = p+1

//line machine.rl:97
act = 50;
	goto st31
	st31:
		if p++; p == pe {
			goto _test_eof31
		}
	st_case_31:
//line machine.go:1065
		switch data[p] {
		case 43:
			goto st1
		case 45:
			goto st1
		case 46:
			goto st3
		case 47:
			goto st2
		case 95:
			goto st1
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr73
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st1
			}
		default:
			goto st1
		}
		goto tr62
	st3:
		if p++; p == pe {
			goto _test_eof3
		}
	st_case_3:
		switch data[p] {
		case 43:
			goto st1
		case 46:
			goto tr5
		case 47:
			goto st2
		case 95:
			goto st1
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto st1
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st1
			}
		default:
			goto st1
		}
		goto tr4
tr73:
//line NONE:1
te = p+1

//line machine.rl:69
act = 27;
	goto st32
	st32:
		if p++; p == pe {
			goto _test_eof32
		}
	st_case_32:
//line machine.go:1131
		switch data[p] {
		case 43:
			goto st1
		case 47:
			goto st2
		case 69:
			goto st4
		case 95:
			goto st1
		case 101:
			goto st4
		}
		switch {
		case data[p] < 48:
			if 45 <= data[p] && data[p] <= 46 {
				goto st1
			}
		case data[p] > 57:
			switch {
			case data[p] > 90:
				if 97 <= data[p] && data[p] <= 122 {
					goto st1
				}
			case data[p] >= 65:
				goto st1
			}
		default:
			goto tr73
		}
		goto tr74
	st4:
		if p++; p == pe {
			goto _test_eof4
		}
	st_case_4:
		switch data[p] {
		case 43:
			goto st5
		case 45:
			goto st5
		case 46:
			goto st1
		case 47:
			goto st2
		case 95:
			goto st1
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr8
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st1
			}
		default:
			goto st1
		}
		goto tr6
	st5:
		if p++; p == pe {
			goto _test_eof5
		}
	st_case_5:
		switch data[p] {
		case 43:
			goto st1
		case 47:
			goto st2
		case 95:
			goto st1
		}
		switch {
		case data[p] < 48:
			if 45 <= data[p] && data[p] <= 46 {
				goto st1
			}
		case data[p] > 57:
			switch {
			case data[p] > 90:
				if 97 <= data[p] && data[p] <= 122 {
					goto st1
				}
			case data[p] >= 65:
				goto st1
			}
		default:
			goto tr8
		}
		goto tr6
tr8:
//line NONE:1
te = p+1

//line machine.rl:69
act = 27;
	goto st33
	st33:
		if p++; p == pe {
			goto _test_eof33
		}
	st_case_33:
//line machine.go:1235
		switch data[p] {
		case 43:
			goto st1
		case 47:
			goto st2
		case 95:
			goto st1
		}
		switch {
		case data[p] < 48:
			if 45 <= data[p] && data[p] <= 46 {
				goto st1
			}
		case data[p] > 57:
			switch {
			case data[p] > 90:
				if 97 <= data[p] && data[p] <= 122 {
					goto st1
				}
			case data[p] >= 65:
				goto st1
			}
		default:
			goto tr8
		}
		goto tr74
tr41:
//line NONE:1
te = p+1

	goto st34
	st34:
		if p++; p == pe {
			goto _test_eof34
		}
	st_case_34:
//line machine.go:1272
		switch data[p] {
		case 42:
			goto st6
		case 43:
			goto st28
		case 47:
			goto tr76
		case 95:
			goto st28
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto st28
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st28
			}
		default:
			goto st28
		}
		goto tr62
	st6:
		if p++; p == pe {
			goto _test_eof6
		}
	st_case_6:
		if data[p] == 42 {
			goto st7
		}
		goto st6
	st7:
		if p++; p == pe {
			goto _test_eof7
		}
	st_case_7:
		switch data[p] {
		case 42:
			goto st7
		case 47:
			goto tr11
		}
		goto st6
tr42:
//line NONE:1
te = p+1

//line machine.rl:70
act = 28;
	goto st35
	st35:
		if p++; p == pe {
			goto _test_eof35
		}
	st_case_35:
//line machine.go:1329
		switch data[p] {
		case 43:
			goto st1
		case 45:
			goto st1
		case 46:
			goto tr73
		case 47:
			goto st2
		case 95:
			goto st1
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr42
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st1
			}
		default:
			goto st1
		}
		goto tr77
tr44:
//line NONE:1
te = p+1

	goto st36
	st36:
		if p++; p == pe {
			goto _test_eof36
		}
	st_case_36:
//line machine.go:1365
		switch data[p] {
		case 43:
			goto st8
		case 61:
			goto tr78
		case 95:
			goto st8
		case 124:
			goto tr79
		}
		switch {
		case data[p] < 48:
			if 45 <= data[p] && data[p] <= 46 {
				goto st8
			}
		case data[p] > 57:
			switch {
			case data[p] > 90:
				if 97 <= data[p] && data[p] <= 122 {
					goto st8
				}
			case data[p] >= 65:
				goto st8
			}
		default:
			goto st8
		}
		goto tr62
	st8:
		if p++; p == pe {
			goto _test_eof8
		}
	st_case_8:
		switch data[p] {
		case 43:
			goto st8
		case 47:
			goto st9
		case 62:
			goto tr14
		case 95:
			goto st8
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto st8
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st8
			}
		default:
			goto st8
		}
		goto tr4
	st9:
		if p++; p == pe {
			goto _test_eof9
		}
	st_case_9:
		switch data[p] {
		case 43:
			goto st8
		case 95:
			goto st8
		}
		switch {
		case data[p] < 48:
			if 45 <= data[p] && data[p] <= 46 {
				goto st8
			}
		case data[p] > 57:
			switch {
			case data[p] > 90:
				if 97 <= data[p] && data[p] <= 122 {
					goto st8
				}
			case data[p] >= 65:
				goto st8
			}
		default:
			goto st8
		}
		goto tr4
	st37:
		if p++; p == pe {
			goto _test_eof37
		}
	st_case_37:
		if data[p] == 61 {
			goto tr80
		}
		goto tr62
	st38:
		if p++; p == pe {
			goto _test_eof38
		}
	st_case_38:
		if data[p] == 61 {
			goto tr81
		}
		goto tr62
tr47:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st39
tr89:
//line NONE:1
te = p+1

//line machine.rl:50
act = 9;
	goto st39
tr92:
//line NONE:1
te = p+1

//line machine.rl:51
act = 10;
	goto st39
tr93:
//line NONE:1
te = p+1

//line machine.rl:52
act = 11;
	goto st39
tr100:
//line NONE:1
te = p+1

//line machine.rl:54
act = 13;
	goto st39
tr102:
//line NONE:1
te = p+1

//line machine.rl:55
act = 14;
	goto st39
tr103:
//line NONE:1
te = p+1

//line machine.rl:56
act = 15;
	goto st39
tr105:
//line NONE:1
te = p+1

//line machine.rl:57
act = 16;
	goto st39
tr108:
//line NONE:1
te = p+1

//line machine.rl:58
act = 17;
	goto st39
tr111:
//line NONE:1
te = p+1

//line machine.rl:59
act = 18;
	goto st39
	st39:
		if p++; p == pe {
			goto _test_eof39
		}
	st_case_39:
//line machine.go:1544
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr0
	st40:
		if p++; p == pe {
			goto _test_eof40
		}
	st_case_40:
		switch data[p] {
		case 39:
			goto st40
		case 45:
			goto st40
		case 95:
			goto st40
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto st40
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st40
			}
		default:
			goto st40
		}
		goto tr83
	st10:
		if p++; p == pe {
			goto _test_eof10
		}
	st_case_10:
		switch data[p] {
		case 43:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto st1
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto st10
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st10
			}
		default:
			goto st10
		}
		goto tr0
	st11:
		if p++; p == pe {
			goto _test_eof11
		}
	st_case_11:
		switch data[p] {
		case 33:
			goto st41
		case 61:
			goto st41
		case 95:
			goto st41
		case 126:
			goto st41
		}
		switch {
		case data[p] < 42:
			if 36 <= data[p] && data[p] <= 39 {
				goto st41
			}
		case data[p] > 58:
			switch {
			case data[p] > 90:
				if 97 <= data[p] && data[p] <= 122 {
					goto st41
				}
			case data[p] >= 63:
				goto st41
			}
		default:
			goto st41
		}
		goto tr0
	st41:
		if p++; p == pe {
			goto _test_eof41
		}
	st_case_41:
		switch data[p] {
		case 33:
			goto st41
		case 61:
			goto st41
		case 95:
			goto st41
		case 126:
			goto st41
		}
		switch {
		case data[p] < 42:
			if 36 <= data[p] && data[p] <= 39 {
				goto st41
			}
		case data[p] > 58:
			switch {
			case data[p] > 90:
				if 97 <= data[p] && data[p] <= 122 {
					goto st41
				}
			case data[p] >= 63:
				goto st41
			}
		default:
			goto st41
		}
		goto tr84
tr49:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st42
	st42:
		if p++; p == pe {
			goto _test_eof42
		}
	st_case_42:
//line machine.go:1704
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st1
		case 46:
			goto st1
		case 47:
			goto st2
		case 95:
			goto tr49
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr49
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr49
			}
		default:
			goto tr49
		}
		goto tr83
tr50:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st43
	st43:
		if p++; p == pe {
			goto _test_eof43
		}
	st_case_43:
//line machine.go:1742
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 115:
			goto tr85
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr85:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st44
	st44:
		if p++; p == pe {
			goto _test_eof44
		}
	st_case_44:
//line machine.go:1784
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 115:
			goto tr86
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr86:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st45
	st45:
		if p++; p == pe {
			goto _test_eof45
		}
	st_case_45:
//line machine.go:1826
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 101:
			goto tr87
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr87:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st46
	st46:
		if p++; p == pe {
			goto _test_eof46
		}
	st_case_46:
//line machine.go:1868
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 114:
			goto tr88
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr88:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st47
	st47:
		if p++; p == pe {
			goto _test_eof47
		}
	st_case_47:
//line machine.go:1910
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 116:
			goto tr89
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr51:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st48
	st48:
		if p++; p == pe {
			goto _test_eof48
		}
	st_case_48:
//line machine.go:1952
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 108:
			goto tr90
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr90:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st49
	st49:
		if p++; p == pe {
			goto _test_eof49
		}
	st_case_49:
//line machine.go:1994
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 115:
			goto tr91
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr91:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st50
	st50:
		if p++; p == pe {
			goto _test_eof50
		}
	st_case_50:
//line machine.go:2036
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 101:
			goto tr92
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr52:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st51
	st51:
		if p++; p == pe {
			goto _test_eof51
		}
	st_case_51:
//line machine.go:2078
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 102:
			goto tr93
		case 110:
			goto tr94
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr94:
//line NONE:1
te = p+1

//line machine.rl:53
act = 12;
	goto st52
	st52:
		if p++; p == pe {
			goto _test_eof52
		}
	st_case_52:
//line machine.go:2122
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 104:
			goto tr96
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr95
tr96:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st53
	st53:
		if p++; p == pe {
			goto _test_eof53
		}
	st_case_53:
//line machine.go:2164
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 101:
			goto tr97
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr97:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st54
	st54:
		if p++; p == pe {
			goto _test_eof54
		}
	st_case_54:
//line machine.go:2206
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 114:
			goto tr98
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr98:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st55
	st55:
		if p++; p == pe {
			goto _test_eof55
		}
	st_case_55:
//line machine.go:2248
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 105:
			goto tr99
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr99:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st56
	st56:
		if p++; p == pe {
			goto _test_eof56
		}
	st_case_56:
//line machine.go:2290
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 116:
			goto tr100
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr53:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st57
	st57:
		if p++; p == pe {
			goto _test_eof57
		}
	st_case_57:
//line machine.go:2332
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 101:
			goto tr101
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr101:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st58
	st58:
		if p++; p == pe {
			goto _test_eof58
		}
	st_case_58:
//line machine.go:2374
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 116:
			goto tr102
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr54:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st59
	st59:
		if p++; p == pe {
			goto _test_eof59
		}
	st_case_59:
//line machine.go:2416
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 114:
			goto tr103
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr55:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st60
	st60:
		if p++; p == pe {
			goto _test_eof60
		}
	st_case_60:
//line machine.go:2458
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 101:
			goto tr104
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr104:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st61
	st61:
		if p++; p == pe {
			goto _test_eof61
		}
	st_case_61:
//line machine.go:2500
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 99:
			goto tr105
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr56:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st62
	st62:
		if p++; p == pe {
			goto _test_eof62
		}
	st_case_62:
//line machine.go:2542
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 104:
			goto tr106
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr106:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st63
	st63:
		if p++; p == pe {
			goto _test_eof63
		}
	st_case_63:
//line machine.go:2584
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 101:
			goto tr107
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr107:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st64
	st64:
		if p++; p == pe {
			goto _test_eof64
		}
	st_case_64:
//line machine.go:2626
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 110:
			goto tr108
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr57:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st65
	st65:
		if p++; p == pe {
			goto _test_eof65
		}
	st_case_65:
//line machine.go:2668
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 105:
			goto tr109
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr109:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st66
	st66:
		if p++; p == pe {
			goto _test_eof66
		}
	st_case_66:
//line machine.go:2710
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 116:
			goto tr110
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
tr110:
//line NONE:1
te = p+1

//line machine.rl:71
act = 29;
	goto st67
	st67:
		if p++; p == pe {
			goto _test_eof67
		}
	st_case_67:
//line machine.go:2752
		switch data[p] {
		case 39:
			goto st40
		case 43:
			goto st10
		case 46:
			goto st10
		case 47:
			goto st2
		case 58:
			goto st11
		case 95:
			goto tr49
		case 104:
			goto tr111
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto tr47
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr47
			}
		default:
			goto tr47
		}
		goto tr83
	st68:
		if p++; p == pe {
			goto _test_eof68
		}
	st_case_68:
		switch data[p] {
		case 62:
			goto tr112
		case 124:
			goto tr113
		}
		goto tr62
tr60:
//line NONE:1
te = p+1

	goto st69
	st69:
		if p++; p == pe {
			goto _test_eof69
		}
	st_case_69:
//line machine.go:2804
		if data[p] == 47 {
			goto st12
		}
		goto tr62
	st12:
		if p++; p == pe {
			goto _test_eof12
		}
	st_case_12:
		switch data[p] {
		case 43:
			goto st70
		case 95:
			goto st70
		}
		switch {
		case data[p] < 48:
			if 45 <= data[p] && data[p] <= 46 {
				goto st70
			}
		case data[p] > 57:
			switch {
			case data[p] > 90:
				if 97 <= data[p] && data[p] <= 122 {
					goto st70
				}
			case data[p] >= 65:
				goto st70
			}
		default:
			goto st70
		}
		goto tr4
	st70:
		if p++; p == pe {
			goto _test_eof70
		}
	st_case_70:
		switch data[p] {
		case 43:
			goto st70
		case 47:
			goto st71
		case 95:
			goto st70
		}
		switch {
		case data[p] < 65:
			if 45 <= data[p] && data[p] <= 57 {
				goto st70
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto st70
			}
		default:
			goto st70
		}
		goto tr115
	st71:
		if p++; p == pe {
			goto _test_eof71
		}
	st_case_71:
		switch data[p] {
		case 43:
			goto st70
		case 95:
			goto st70
		}
		switch {
		case data[p] < 48:
			if 45 <= data[p] && data[p] <= 46 {
				goto st70
			}
		case data[p] > 57:
			switch {
			case data[p] > 90:
				if 97 <= data[p] && data[p] <= 122 {
					goto st70
				}
			case data[p] >= 65:
				goto st70
			}
		default:
			goto st70
		}
		goto tr115
tr19:
//line machine.rl:32
p = (te) - 1
{ tok(text); addLines() }
	goto st72
tr21:
//line NONE:1
	switch act {
	case 0:
	{{goto st0 }}
	case 3:
	{p = (te) - 1
 tok(text); addLines() }
	}
	
	goto st72
tr117:
//line machine.rl:30
te = p+1
{ if !tokleave('"') { return }; {top--; cs = stack[top];goto _again } }
	goto st72
tr120:
//line machine.rl:32
te = p
p--
{ tok(text); addLines() }
	goto st72
tr122:
//line machine.rl:33
te = p
p--
{ tok(text) }
	goto st72
tr123:
//line machine.rl:31
te = p+1
{ tokenter(interp, '}'); { if nostack() { return }; {stack[top] = 72; top++; goto st19 }} }
	goto st72
	st72:
//line NONE:1
ts = 0

//line NONE:1
act = 0

		if p++; p == pe {
			goto _test_eof72
		}
	st_case_72:
//line NONE:1
ts = p

//line machine.go:2945
		switch data[p] {
		case 34:
			goto tr117
		case 36:
			goto st74
		case 92:
			goto st14
		}
		goto tr20
tr20:
//line NONE:1
te = p+1

//line machine.rl:32
act = 3;
	goto st73
	st73:
		if p++; p == pe {
			goto _test_eof73
		}
	st_case_73:
//line machine.go:2967
		switch data[p] {
		case 34:
			goto tr120
		case 36:
			goto st13
		case 92:
			goto st14
		}
		goto tr20
	st13:
		if p++; p == pe {
			goto _test_eof13
		}
	st_case_13:
		switch data[p] {
		case 34:
			goto tr19
		case 123:
			goto tr19
		}
		goto tr20
	st14:
		if p++; p == pe {
			goto _test_eof14
		}
	st_case_14:
		goto tr20
	st74:
		if p++; p == pe {
			goto _test_eof74
		}
	st_case_74:
		switch data[p] {
		case 34:
			goto tr122
		case 123:
			goto tr123
		}
		goto tr20
tr22:
//line machine.rl:42
p = (te) - 1
{ tok(text); addLines() }
	goto st75
tr26:
//line NONE:1
	switch act {
	case 5:
	{p = (te) - 1
 if !tokleave(ii) { return }; {top--; cs = stack[top];goto _again } }
	case 7:
	{p = (te) - 1
 tok(text); addLines() }
	}
	
	goto st75
tr126:
//line machine.rl:42
te = p
p--
{ tok(text); addLines() }
	goto st75
tr129:
//line machine.rl:43
te = p
p--
{ tok(text) }
	goto st75
tr130:
//line machine.rl:41
te = p+1
{ tokenter(interp, '}'); { if nostack() { return }; {stack[top] = 75; top++; goto st19 }} }
	goto st75
tr132:
//line machine.rl:40
te = p
p--
{ if !tokleave(ii) { return }; {top--; cs = stack[top];goto _again } }
	goto st75
	st75:
//line NONE:1
ts = 0

		if p++; p == pe {
			goto _test_eof75
		}
	st_case_75:
//line NONE:1
ts = p

//line machine.go:3058
		switch data[p] {
		case 36:
			goto st77
		case 39:
			goto st78
		}
		goto tr23
tr23:
//line NONE:1
te = p+1

//line machine.rl:42
act = 7;
	goto st76
	st76:
		if p++; p == pe {
			goto _test_eof76
		}
	st_case_76:
//line machine.go:3078
		switch data[p] {
		case 36:
			goto st15
		case 39:
			goto st16
		}
		goto tr23
	st15:
		if p++; p == pe {
			goto _test_eof15
		}
	st_case_15:
		switch data[p] {
		case 39:
			goto tr22
		case 123:
			goto tr22
		}
		goto tr23
	st16:
		if p++; p == pe {
			goto _test_eof16
		}
	st_case_16:
		switch data[p] {
		case 36:
			goto tr22
		case 39:
			goto st17
		}
		goto tr23
	st17:
		if p++; p == pe {
			goto _test_eof17
		}
	st_case_17:
		switch data[p] {
		case 36:
			goto tr23
		case 39:
			goto tr23
		case 92:
			goto st18
		}
		goto tr22
	st18:
		if p++; p == pe {
			goto _test_eof18
		}
	st_case_18:
		goto tr23
	st77:
		if p++; p == pe {
			goto _test_eof77
		}
	st_case_77:
		switch data[p] {
		case 39:
			goto tr129
		case 123:
			goto tr130
		}
		goto tr23
	st78:
		if p++; p == pe {
			goto _test_eof78
		}
	st_case_78:
		switch data[p] {
		case 36:
			goto tr129
		case 39:
			goto tr131
		}
		goto tr23
tr131:
//line NONE:1
te = p+1

//line machine.rl:40
act = 5;
	goto st79
	st79:
		if p++; p == pe {
			goto _test_eof79
		}
	st_case_79:
//line machine.go:3166
		switch data[p] {
		case 36:
			goto tr23
		case 39:
			goto tr23
		case 92:
			goto st18
		}
		goto tr132
st_case_0:
	st0:
		cs = 0
		goto _out
	st_out:
	_test_eof19: cs = 19; goto _test_eof
	_test_eof20: cs = 20; goto _test_eof
	_test_eof21: cs = 21; goto _test_eof
	_test_eof22: cs = 22; goto _test_eof
	_test_eof23: cs = 23; goto _test_eof
	_test_eof24: cs = 24; goto _test_eof
	_test_eof25: cs = 25; goto _test_eof
	_test_eof26: cs = 26; goto _test_eof
	_test_eof27: cs = 27; goto _test_eof
	_test_eof1: cs = 1; goto _test_eof
	_test_eof2: cs = 2; goto _test_eof
	_test_eof28: cs = 28; goto _test_eof
	_test_eof29: cs = 29; goto _test_eof
	_test_eof30: cs = 30; goto _test_eof
	_test_eof31: cs = 31; goto _test_eof
	_test_eof3: cs = 3; goto _test_eof
	_test_eof32: cs = 32; goto _test_eof
	_test_eof4: cs = 4; goto _test_eof
	_test_eof5: cs = 5; goto _test_eof
	_test_eof33: cs = 33; goto _test_eof
	_test_eof34: cs = 34; goto _test_eof
	_test_eof6: cs = 6; goto _test_eof
	_test_eof7: cs = 7; goto _test_eof
	_test_eof35: cs = 35; goto _test_eof
	_test_eof36: cs = 36; goto _test_eof
	_test_eof8: cs = 8; goto _test_eof
	_test_eof9: cs = 9; goto _test_eof
	_test_eof37: cs = 37; goto _test_eof
	_test_eof38: cs = 38; goto _test_eof
	_test_eof39: cs = 39; goto _test_eof
	_test_eof40: cs = 40; goto _test_eof
	_test_eof10: cs = 10; goto _test_eof
	_test_eof11: cs = 11; goto _test_eof
	_test_eof41: cs = 41; goto _test_eof
	_test_eof42: cs = 42; goto _test_eof
	_test_eof43: cs = 43; goto _test_eof
	_test_eof44: cs = 44; goto _test_eof
	_test_eof45: cs = 45; goto _test_eof
	_test_eof46: cs = 46; goto _test_eof
	_test_eof47: cs = 47; goto _test_eof
	_test_eof48: cs = 48; goto _test_eof
	_test_eof49: cs = 49; goto _test_eof
	_test_eof50: cs = 50; goto _test_eof
	_test_eof51: cs = 51; goto _test_eof
	_test_eof52: cs = 52; goto _test_eof
	_test_eof53: cs = 53; goto _test_eof
	_test_eof54: cs = 54; goto _test_eof
	_test_eof55: cs = 55; goto _test_eof
	_test_eof56: cs = 56; goto _test_eof
	_test_eof57: cs = 57; goto _test_eof
	_test_eof58: cs = 58; goto _test_eof
	_test_eof59: cs = 59; goto _test_eof
	_test_eof60: cs = 60; goto _test_eof
	_test_eof61: cs = 61; goto _test_eof
	_test_eof62: cs = 62; goto _test_eof
	_test_eof63: cs = 63; goto _test_eof
	_test_eof64: cs = 64; goto _test_eof
	_test_eof65: cs = 65; goto _test_eof
	_test_eof66: cs = 66; goto _test_eof
	_test_eof67: cs = 67; goto _test_eof
	_test_eof68: cs = 68; goto _test_eof
	_test_eof69: cs = 69; goto _test_eof
	_test_eof12: cs = 12; goto _test_eof
	_test_eof70: cs = 70; goto _test_eof
	_test_eof71: cs = 71; goto _test_eof
	_test_eof72: cs = 72; goto _test_eof
	_test_eof73: cs = 73; goto _test_eof
	_test_eof13: cs = 13; goto _test_eof
	_test_eof14: cs = 14; goto _test_eof
	_test_eof74: cs = 74; goto _test_eof
	_test_eof75: cs = 75; goto _test_eof
	_test_eof76: cs = 76; goto _test_eof
	_test_eof15: cs = 15; goto _test_eof
	_test_eof16: cs = 16; goto _test_eof
	_test_eof17: cs = 17; goto _test_eof
	_test_eof18: cs = 18; goto _test_eof
	_test_eof77: cs = 77; goto _test_eof
	_test_eof78: cs = 78; goto _test_eof
	_test_eof79: cs = 79; goto _test_eof

	_test_eof: {}
	if p == eof {
		switch cs {
		case 20:
			goto tr61
		case 21:
			goto tr62
		case 22:
			goto tr64
		case 23:
			goto tr62
		case 24:
			goto tr62
		case 25:
			goto tr62
		case 26:
			goto tr62
		case 27:
			goto tr0
		case 1:
			goto tr0
		case 2:
			goto tr0
		case 28:
			goto tr69
		case 29:
			goto tr69
		case 30:
			goto tr62
		case 31:
			goto tr62
		case 3:
			goto tr4
		case 32:
			goto tr74
		case 4:
			goto tr6
		case 5:
			goto tr6
		case 33:
			goto tr74
		case 34:
			goto tr62
		case 6:
			goto tr4
		case 7:
			goto tr4
		case 35:
			goto tr77
		case 36:
			goto tr62
		case 8:
			goto tr4
		case 9:
			goto tr4
		case 37:
			goto tr62
		case 38:
			goto tr62
		case 39:
			goto tr0
		case 40:
			goto tr83
		case 10:
			goto tr0
		case 11:
			goto tr0
		case 41:
			goto tr84
		case 42:
			goto tr83
		case 43:
			goto tr83
		case 44:
			goto tr83
		case 45:
			goto tr83
		case 46:
			goto tr83
		case 47:
			goto tr83
		case 48:
			goto tr83
		case 49:
			goto tr83
		case 50:
			goto tr83
		case 51:
			goto tr83
		case 52:
			goto tr95
		case 53:
			goto tr83
		case 54:
			goto tr83
		case 55:
			goto tr83
		case 56:
			goto tr83
		case 57:
			goto tr83
		case 58:
			goto tr83
		case 59:
			goto tr83
		case 60:
			goto tr83
		case 61:
			goto tr83
		case 62:
			goto tr83
		case 63:
			goto tr83
		case 64:
			goto tr83
		case 65:
			goto tr83
		case 66:
			goto tr83
		case 67:
			goto tr83
		case 68:
			goto tr62
		case 69:
			goto tr62
		case 12:
			goto tr4
		case 70:
			goto tr115
		case 71:
			goto tr115
		case 73:
			goto tr120
		case 13:
			goto tr19
		case 14:
			goto tr21
		case 74:
			goto tr122
		case 76:
			goto tr126
		case 15:
			goto tr22
		case 16:
			goto tr22
		case 17:
			goto tr22
		case 18:
			goto tr26
		case 77:
			goto tr129
		case 78:
			goto tr129
		case 79:
			goto tr132
		}
	}

	_out: {}
	}

//line machine.rl:172

	if p != eof {
		err = r.Errorf("precedes the token that failed to lex")
	} else if len(backrefs) != 0 {
		iprev, _ := backrefs.Pop()
		err = &LexerError{Pos: r.TokenPos(iprev), Desc: fmt.Sprintf("%s is not terminated", r.TokenSymString(iprev))}
	}
	return
}
