package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestRequestHeader(t *testing.T) {
	testRequestStrings := []string{
		"GET hosts\n\n",
		"GET hosts\nColumns: name state\n\n",
		"GET hosts\nColumns: name state\nFilter: state != 1\n\n",
		"GET hosts\nOutputFormat: wrapped_json\nColumnHeaders: on\n\n",
		"GET hosts\nResponseHeader: fixed16\n\n",
		"GET hosts\nColumns: name state\nFilter: state != 1\nFilter: is_executing = 1\nOr: 2\n\n",
		"GET hosts\nColumns: name state\nFilter: state != 1\nFilter: is_executing = 1\nAnd: 2\nFilter: state = 1\nOr: 2\nFilter: name = test\n\n",
		"GET hosts\nBackends: mockid0\n\n",
		"GET hosts\nLimit: 25\nOffset: 5\n\n",
		"GET hosts\nSort: name asc\nSort: state desc\n\n",
		"GET hosts\nStats: state = 1\nStats: avg latency\nStats: state = 3\nStats: state != 1\nStatsAnd: 2\n\n",
		"GET hosts\nColumns: name\nFilter: name ~~ test\n\n",
		"GET hosts\nColumns: name\nFilter: name !~ Test\n\n",
		"GET hosts\nColumns: name\nFilter: name !~~ test\n\n",
		"GET hosts\nColumns: name\nFilter: custom_variables ~~ TAGS test\n\n",
		"GET hosts\nColumns: name\nFilter: custom_variables = TAGS\n\n",
		"GET hosts\nColumns: name\nFilter: name !=\n\n",
		"COMMAND [123456] TEST\n\n",
		"GET hosts\nColumns: name\nFilter: name = test\nWaitTrigger: all\nWaitObject: test\nWaitTimeout: 10000\nWaitCondition: last_check > 1473760401\n\n",
		"GET hosts\nColumns: name\nFilter: latency != 1.23456789012345\n\n",
		"GET hosts\nColumns: name comments\nFilter: comments >= 1\n\n",
		"GET hosts\nColumns: name contact_groups\nFilter: contact_groups >= test\n\n",
		"GET hosts\nColumns: name\nFilter: last_check >= 123456789\n\n",
		"GET hosts\nColumns: name\nFilter: last_check =\n\n",
		"GET hosts\nAuthUser: testUser\n\n",
	}
	for _, str := range testRequestStrings {
		buf := bufio.NewReader(bytes.NewBufferString(str))
		req, _, err := NewRequest(buf)
		if err != nil {
			t.Fatal(err)
		}
		if err = assertEq(str, req.String()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRequestHeaderTable(t *testing.T) {
	buf := bufio.NewReader(bytes.NewBufferString("GET hosts\n"))
	req, _, _ := NewRequest(buf)
	if err := assertEq("hosts", req.Table); err != nil {
		t.Fatal(err)
	}
}

func TestRequestHeaderLimit(t *testing.T) {
	buf := bufio.NewReader(bytes.NewBufferString("GET hosts\nLimit: 10\n"))
	req, _, _ := NewRequest(buf)
	if err := assertEq(10, *req.Limit); err != nil {
		t.Fatal(err)
	}
}

func TestRequestHeaderOffset(t *testing.T) {
	buf := bufio.NewReader(bytes.NewBufferString("GET hosts\nOffset: 3\n"))
	req, _, _ := NewRequest(buf)
	if err := assertEq(3, req.Offset); err != nil {
		t.Fatal(err)
	}
}

func TestRequestHeaderColumns(t *testing.T) {
	buf := bufio.NewReader(bytes.NewBufferString("GET hosts\nColumns: name state\n"))
	req, _, _ := NewRequest(buf)
	if err := assertEq([]string{"name", "state"}, req.Columns); err != nil {
		t.Fatal(err)
	}
}

func TestRequestHeaderSort(t *testing.T) {
	req, _, _ := NewRequest(bufio.NewReader(bytes.NewBufferString("GET hosts\nColumns: latency state name\nSort: name desc\nSort: state asc\n")))
	if err := assertEq(&SortField{Name: "name", Direction: Desc, Index: 0, Column: Objects.Tables["hosts"].GetColumn("name")}, req.Sort[0]); err != nil {
		t.Fatal(err)
	}
	if err := assertEq(&SortField{Name: "state", Direction: Asc, Index: 0, Column: Objects.Tables["hosts"].GetColumn("state")}, req.Sort[1]); err != nil {
		t.Fatal(err)
	}
}

func TestRequestHeaderSortCust(t *testing.T) {
	req, _, _ := NewRequest(bufio.NewReader(bytes.NewBufferString("GET hosts\nColumns: name custom_variables\nSort: custom_variables TEST asc\n")))
	if err := assertEq(&SortField{Name: "custom_variables", Direction: Asc, Index: 0, Args: "TEST", Column: Objects.Tables["hosts"].GetColumn("custom_variables")}, req.Sort[0]); err != nil {
		t.Fatal(err)
	}
}

func TestRequestHeaderFilter1(t *testing.T) {
	buf := bufio.NewReader(bytes.NewBufferString("GET hosts\nFilter: name != test\n"))
	req, _, _ := NewRequest(buf)
	if err := assertEq(len(req.Filter), 1); err != nil {
		t.Fatal(err)
	}
	if err := assertEq(req.Filter[0].Column.Name, "name"); err != nil {
		t.Fatal(err)
	}
}

func TestRequestHeaderFilter2(t *testing.T) {
	buf := bufio.NewReader(bytes.NewBufferString("GET hosts\nFilter: state != 1\nFilter: name = with spaces \n"))
	req, _, _ := NewRequest(buf)
	if err := assertEq(len(req.Filter), 2); err != nil {
		t.Fatal(err)
	}
	if err := assertEq(req.Filter[0].Column.Name, "state"); err != nil {
		t.Fatal(err)
	}
	if err := assertEq(req.Filter[1].Column.Name, "name"); err != nil {
		t.Fatal(err)
	}
	if err := assertEq(req.Filter[1].StrValue, "with spaces"); err != nil {
		t.Fatal(err)
	}
}

func TestRequestHeaderFilter3(t *testing.T) {
	buf := bufio.NewReader(bytes.NewBufferString("GET hosts\nFilter: state != 1\nFilter: name = with spaces\nOr: 2"))
	req, _, _ := NewRequest(buf)
	if err := assertEq(len(req.Filter), 1); err != nil {
		t.Fatal(err)
	}
	if err := assertEq(len(req.Filter[0].Filter), 2); err != nil {
		t.Fatal(err)
	}
	if err := assertEq(req.Filter[0].GroupOperator, Or); err != nil {
		t.Fatal(err)
	}
}

func TestRequestListFilter(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET hosts\nColumns: name\nFilter: contact_groups >= example\nSort: name asc")
	if err != nil {
		t.Fatal(err)
	}
	if err := assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Fatal(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestHeaderMultipleCommands(t *testing.T) {
	buf := bufio.NewReader(bytes.NewBufferString(`COMMAND [1473627610] SCHEDULE_FORCED_SVC_CHECK;demo;Web1;1473627610
Backends: mockid0

COMMAND [1473627610] SCHEDULE_FORCED_SVC_CHECK;demo;Web2;1473627610`))
	req, size, err := NewRequest(buf)
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(size, 87); err != nil {
		t.Fatal(err)
	}
	if err = assertEq(req.Command, "COMMAND [1473627610] SCHEDULE_FORCED_SVC_CHECK;demo;Web1;1473627610"); err != nil {
		t.Fatal(err)
	}
	if err = assertEq(req.Backends[0], "mockid0"); err != nil {
		t.Fatal(err)
	}
	req, size, err = NewRequest(buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := assertEq(size, 67); err != nil {
		t.Fatal(err)
	}
	if err := assertEq(req.Command, "COMMAND [1473627610] SCHEDULE_FORCED_SVC_CHECK;demo;Web2;1473627610"); err != nil {
		t.Fatal(err)
	}
}

type ErrorRequest struct {
	Request string
	Error   string
}

func TestResponseErrorsFunc(t *testing.T) {
	peer := StartTestPeer(1, 0, 0)
	PauseTestPeers(peer)

	testRequestStrings := []ErrorRequest{
		{"", "bad request: empty request"},
		{"NOE", "bad request: NOE"},
		{"GET none\nColumns: none", "bad request: table none does not exist"},
		{"GET hosts\nnone", "bad request: syntax error in: none"},
		{"GET hosts\nNone: blah", "bad request: unrecognized header in: None: blah"},
		{"GET hosts\nLimit: x", "bad request: expecting a positive number in: Limit: x"},
		{"GET hosts\nLimit: -1", "bad request: expecting a positive number in: Limit: -1"},
		{"GET hosts\nOffset: x", "bad request: expecting a positive number in: Offset: x"},
		{"GET hosts\nOffset: -1", "bad request: expecting a positive number in: Offset: -1"},
		{"GET hosts\nSort: name none", "bad request: unrecognized sort direction, must be asc or desc in: Sort: name none"},
		{"GET hosts\nResponseheader: none", "bad request: unrecognized responseformat, only fixed16 is supported in: Responseheader: none"},
		{"GET hosts\nOutputFormat: csv: none", "bad request: unrecognized outputformat, choose from json, wrapped_json and python in: OutputFormat: csv: none"},
		{"GET hosts\nStatsAnd: 1", "bad request: not enough filter on stack in: StatsAnd: 1"},
		{"GET hosts\nStatsOr: 1", "bad request: not enough filter on stack in: StatsOr: 1"},
		{"GET hosts\nFilter: name", "bad request: filter header must be Filter: <field> <operator> <value> in: Filter: name"},
		{"GET hosts\nFilter: name ~~ *^", "bad request: invalid regular expression: error parsing regexp: missing argument to repetition operator: `*` in: Filter: name ~~ *^"},
		{"GET hosts\nStats: name", "bad request: stats header, must be Stats: <field> <operator> <value> OR Stats: <sum|avg|min|max> <field> in: Stats: name"},
		{"GET hosts\nStats: avg none", "bad request: unrecognized column from stats: none in: Stats: avg none"},
		{"GET hosts\nFilter: name !=\nAnd: x", "bad request: And must be a positive number in: And: x"},
		{"GET hosts\nColumns: name\nFilter: custom_variables =", "bad request: custom variable filter must have form \"Filter: custom_variables <op> <variable> [<value>]\" in: Filter: custom_variables ="},
		{"GET hosts\nKeepalive: broke", "bad request: must be 'on' or 'off' in: Keepalive: broke"},
	}

	for _, er := range testRequestStrings {
		_, _, err := peer.QueryString(er.Request)
		if err == nil {
			t.Fatalf("No Error in Request: " + er.Request)
		}
		if err = assertEq(er.Error, err.Error()); err != nil {
			t.Error("Request: " + er.Request)
			t.Fatalf(err.Error())
		}
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestNestedFilter(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	if err := assertEq(1, len(PeerMap)); err != nil {
		t.Error(err)
	}

	query := `GET services
Columns: host_name description state peer_key
Filter: description ~~ testsvc_1
Filter: display_name ~~ testsvc_1
Or: 2
Filter: host_name !~~ testhost_1
Filter: host_name !~~ testhost_[2-6]
And: 2
And: 2
Limit: 100
Offset: 0
Sort: host_name asc
Sort: description asc
OutputFormat: wrapped_json
ResponseHeader: fixed16
`
	res, _, err := peer.QueryString(query)
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(3, len(*res)); err != nil {
		t.Fatal(err)
	}

	if err = assertEq("testhost_7", (*res)[0][0]); err != nil {
		t.Error(err)
	}
	if err = assertEq("testsvc_1", (*res)[0][1]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestStats(t *testing.T) {
	peer := StartTestPeer(4, 10, 10)
	PauseTestPeers(peer)

	if err := assertEq(4, len(PeerMap)); err != nil {
		t.Error(err)
	}

	res, _, err := peer.QueryString("GET hosts\nColumns: name latency\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(40, len(*res)); err != nil {
		t.Error(err)
	}

	res, _, err = peer.QueryString("GET hosts\nStats: sum latency\nStats: avg latency\nStats: min has_been_checked\nStats: max execution_time\nStats: name !=\n")
	if err != nil {
		t.Fatal(err)
	}

	if err = assertEq(3.346320092680001, (*res)[0][0]); err != nil {
		t.Error(err)
	}
	if err = assertEq(0.08365800231700002, (*res)[0][1]); err != nil {
		t.Error(err)
	}
	if err = assertEq(float64(1), (*res)[0][2]); err != nil {
		t.Error(err)
	}
	if err = assertEq(0.005645, (*res)[0][3]); err != nil {
		t.Error(err)
	}
	if err = assertEq(float64(40), (*res)[0][4]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestStatsGroupBy(t *testing.T) {
	peer := StartTestPeer(4, 10, 10)
	PauseTestPeers(peer)

	if err := assertEq(4, len(PeerMap)); err != nil {
		t.Error(err)
	}

	res, _, err := peer.QueryString("GET hosts\nColumns: name\nStats: avg latency\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(10, len(*res)); err != nil {
		t.Error(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}
	if err = assertEq(0.083658002317, (*res)[1][1]); err != nil {
		t.Error(err)
	}

	res, _, err = peer.QueryString("GET hosts\nColumns: name alias\nStats: avg latency\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(10, len(*res)); err != nil {
		t.Error(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}
	if err = assertEq("authhost", (*res)[1][1]); err != nil {
		t.Error(err)
	}
	if err = assertEq(0.083658002317, (*res)[1][2]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestStatsEmpty(t *testing.T) {
	peer := StartTestPeer(2, 0, 0)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET hosts\nFilter: check_type = 15\nStats: sum percent_state_change\nStats: min percent_state_change\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(1, len(*res)); err != nil {
		t.Fatal(err)
	}
	if err = assertEq(float64(0), (*res)[0][0]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestStatsBroken(t *testing.T) {
	peer := StartTestPeer(1, 0, 0)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET hosts\nStats: sum name\nStats: avg contacts\nStats: min plugin_output\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(float64(0), (*res)[0][0]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestRefs(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	res1, _, err := peer.QueryString("GET hosts\nColumns: name latency check_command\nLimit: 1\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(1, len(*res1)); err != nil {
		t.Error(err)
	}

	res2, _, err := peer.QueryString("GET services\nColumns: host_name host_latency host_check_command\nFilter: host_name = " + (*res1)[0][0].(string) + "\nLimit: 1\n\n")
	if err != nil {
		t.Fatal(err)
	}

	if err = assertEq((*res1)[0], (*res2)[0]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestBrokenColumns(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET hosts\nColumns: host_name alias\nFilter: host_name = testhost_1\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(1, len(*res)); err != nil {
		t.Fatal(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}
	if err = assertEq("localhost", (*res)[0][1]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestGroupByTable(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET servicesbyhostgroup\nColumns: host_name description host_groups groups host_alias host_address\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(10, len(*res)); err != nil {
		t.Fatal(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}
	if err = assertEq("127.0.0.1", (*res)[0][5]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestBlocking(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	start := time.Now()

	// start long running query in background
	errs := make(chan error, 1)
	go func() {
		_, _, err := peer.QueryString("GET hosts\nColumns: name latency check_command\nLimit: 1\nWaitTrigger: all\nWaitTimeout: 5000\nWaitCondition: state = 99\n\n")
		errs <- err
	}()

	// test how long next query will take
	_, _, err1 := peer.QueryString("GET hosts\nColumns: name latency check_command\nLimit: 1\n\n")
	if err1 != nil {
		t.Fatal(err1)
	}

	elapsed := time.Since(start)
	if elapsed.Seconds() > 3 {
		t.Error("query2 should return immediately")
	}

	// check non-blocking if there were any errors in the long running query so far
	select {
	case err2 := <-errs:
		if err2 != nil {
			t.Fatal(err2)
		}
	default:
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestSort(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET hosts\nColumns: name latency\nSort: latency asc\nLimit: 5\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(5, len(*res)); err != nil {
		t.Error(err)
	}
	if err = assertEq("testhost_4", (*res)[0][0]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestSortColumnNotRequested(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET hosts\nColumns: name state alias\nSort: latency asc\nSort: name asc\nLimit: 5\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(5, len(*res)); err != nil {
		t.Error(err)
	}
	if err = assertEq(3, len((*res)[0])); err != nil {
		t.Error(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestNoColumns(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET status\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(2, len(*res)); err != nil {
		t.Error(err)
	}
	if err = assertEq(49, len((*res)[0])); err != nil {
		t.Error(err)
	}
	if err = assertEq("program_start", (*res)[0][0]); err != nil {
		t.Error(err)
	}
	if err = assertEq("mockid0", (*res)[1][36]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestUnknownOptionalColumns(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET hosts\nColumns: name is_impact\nLimit: 1\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(1, len(*res)); err != nil {
		t.Fatal(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}
	if err = assertEq(float64(-1), (*res)[0][1]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestUnknownOptionalRefsColumns(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET services\nColumns: host_name host_is_impact\nLimit: 1\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(1, len(*res)); err != nil {
		t.Fatal(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}
	if err = assertEq(float64(-1), (*res)[0][1]); err != nil {
		t.Error(err)
	}

	res, _, err = peer.QueryString("GET services\nColumns: host_name\nFilter: host_is_impact != -1\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(0, len(*res)); err != nil {
		t.Fatal(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestColumnsWrappedJson(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET hosts\nColumns: name state alias\nOutputFormat: wrapped_json\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(10, len(*res)); err != nil {
		t.Error(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}

	res, meta, err := peer.QueryString("GET hosts\nColumns: name state alias\nOutputFormat: wrapped_json\nColumnHeaders: on\nLimit: 5\n\n")
	if err != nil {
		t.Fatal(err)
	}
	var jsonTest interface{}
	jErr := json.Unmarshal(*peer.lastResponse, &jsonTest)
	if jErr != nil {
		t.Fatal(jErr)
	}
	if err = assertEq(5, len(*res)); err != nil {
		t.Error(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}
	if err = assertEq(int64(10), meta.Total); err != nil {
		t.Error(err)
	}
	if err = assertEq("name", meta.Columns[0]); err != nil {
		t.Error(err)
	}

	res, _, err = peer.QueryString("GET hosts\nColumns: name state alias\nOutputFormat: json\n\n")
	if err != nil {
		t.Fatal(err)
	}
	jErr = json.Unmarshal(*peer.lastResponse, &jsonTest)
	if jErr != nil {
		t.Fatal(jErr)
	}
	if err = assertEq(10, len(*res)); err != nil {
		t.Error(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestCommands(t *testing.T) {
	peer := StartTestPeer(1, 10, 10)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("COMMAND [0] test_ok\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Errorf("result for successful command should be empty")
	}

	res, _, err = peer.QueryString("COMMAND [0] test_broken\n\n")
	if err == nil {
		t.Fatal("expected error for broken command")
	}
	if res != nil {
		t.Errorf("result for unsuccessful command should be empty")
	}
	if err2 := assertEq(err.Error(), "command broken"); err2 != nil {
		t.Error(err2)
	}
	if err2 := assertEq(err.(*PeerCommandError).code, 400); err2 != nil {
		t.Error(err2)
	}

	_, _, err = peer.QueryString("COMMAND [123.456] test_broken\n\n")
	if err == nil {
		t.Fatal("expected error for broken command")
	}
	if err2 := assertEq(err.Error(), "bad request: COMMAND [123.456] test_broken"); err2 != nil {
		t.Error(err2)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestHTTPCommands(t *testing.T) {
	peer, cleanup := GetHTTPMockServerPeer(t)
	defer cleanup()

	res, _, err := peer.QueryString("COMMAND [0] test_ok")
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Errorf("result for successful command should be empty")
	}

	res, _, err = peer.QueryString("COMMAND [0] test_broken")
	if err == nil {
		t.Fatal("expected error for broken command")
	}
	if res != nil {
		t.Errorf("result for unsuccessful command should be empty")
	}
	if err2 := assertEq("command broken", err.Error()); err2 != nil {
		t.Error(err2)
	}
	if err2 := assertEq(400, err.(*PeerCommandError).code); err2 != nil {
		t.Error(err2)
	}
	if err2 := assertEq(2.20, peer.StatusGet("ThrukVersion")); err2 != nil {
		t.Errorf("version set correctly: %s", err2.Error())
	}

	// newer thruk versions return result directly
	thrukVersion := 2.26
	peer.StatusSet("ThrukVersion", thrukVersion)

	res, _, err = peer.QueryString("COMMAND [0] test_ok")
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Errorf("result for successful command should be empty")
	}

	res, _, err = peer.QueryString("COMMAND [0] test_broken")
	if err == nil {
		t.Fatal("expected error for broken command")
	}
	if res != nil {
		t.Errorf("result for unsuccessful command should be empty")
	}
	if err2 := assertEq("command broken", err.Error()); err2 != nil {
		t.Error(err2)
	}
	if err2 := assertEq(400, err.(*PeerCommandError).code); err2 != nil {
		t.Error(err2)
	}
	if err2 := assertEq(thrukVersion, peer.StatusGet("ThrukVersion")); err2 != nil {
		t.Errorf("version unchanged: %s", err2.Error())
	}
}

func TestHTTPPeer(t *testing.T) {
	peer, cleanup := GetHTTPMockServerPeer(t)
	defer cleanup()

	ok := peer.InitAllTables()
	if err := assertEq(ok, true); err != nil {
		t.Error(err)
	}
}

func TestRequestPassthrough(t *testing.T) {
	peer := StartTestPeer(5, 10, 10)
	PauseTestPeers(peer)

	// query without virtual columns
	res, _, err := peer.QueryString("GET log\nColumns: time type message\nLimit: 3\nSort: time asc\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(3, len(*res)); err != nil {
		t.Fatal(err)
	}
	if err = assertEq(float64(1558468664), (*res)[0][0]); err != nil {
		t.Fatal(err)
	}
	if err = assertEq("HOST ALERT", (*res)[0][1]); err != nil {
		t.Fatal(err)
	}

	// query with extra virtual columns
	res, _, err = peer.QueryString("GET log\nColumns: time peer_key type message\nLimit: 3\nSort: peer_key asc\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(3, len(*res)); err != nil {
		t.Fatal(err)
	}
	if err = assertEq(float64(1558468664), (*res)[0][0]); err != nil {
		t.Fatal(err)
	}
	if err = assertEq("mockid0", (*res)[0][1]); err != nil {
		t.Fatal(err)
	}
	if err = assertEq("HOST ALERT", (*res)[0][2]); err != nil {
		t.Fatal(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

func TestRequestSites(t *testing.T) {
	extraConfig := `
	Listen = ["test.sock"]

	[[Connections]]
	name   = 'offline1'
	id     = 'offline1'
	source = ['/does/not/exist.sock']

	[[Connections]]
	name   = 'offline2'
	id     = 'offline2'
	source = ['/does/not/exist.sock']
	`
	peer := StartTestPeerExtra(4, 10, 10, extraConfig)
	PauseTestPeers(peer)

	res, _, err := peer.QueryString("GET sites\nColumns: name status last_error\nSort: name")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq(6, len(*res)); err != nil {
		t.Fatal(err)
	}
	if err = assertEq("offline2", (*res)[5][0]); err != nil {
		t.Fatal(err)
	}
	if err = assertEq(float64(2), (*res)[5][1]); err != nil {
		t.Fatal(err)
	}
	if err = assertLike("connect: no such file or directory", (*res)[5][2].(string)); err != nil {
		t.Fatal(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}

/* Tests that getting columns based on <table>_<colum-name> works */
func TestTableNameColName(t *testing.T) {
	peer := StartTestPeer(1, 2, 2)
	PauseTestPeers(peer)

	if err := assertEq(1, len(PeerMap)); err != nil {
		t.Error(err)
	}

	res, _, err := peer.QueryString("GET hosts\nColumns: host_name\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}

	res, _, err = peer.QueryString("GET hostgroups\nColumns: hostgroup_name\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq("Everything", (*res)[0][0]); err != nil {
		t.Error(err)
	}

	res, _, err = peer.QueryString("GET hostgroups\nColumns: hostgroup_name\nFilter: hostgroup_name = host_1\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq("host_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}

	res, _, err = peer.QueryString("GET hostsbygroup\nColumns: host_name\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq("testhost_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}

	res, _, err = peer.QueryString("GET servicesbygroup\nColumns: service_description\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = assertEq("testsvc_1", (*res)[0][0]); err != nil {
		t.Error(err)
	}

	if err := StopTestPeer(peer); err != nil {
		panic(err.Error())
	}
}
