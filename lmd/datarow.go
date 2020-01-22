package main

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/lkarlslund/stringdedup"
)

const ListSepChar1 = string(0)

// DataRow represents a single entry in a DataTable
type DataRow struct {
	noCopy                noCopy                 // we don't want to make copies, use references
	DataStore             *DataStore             // reference to the datastore itself
	Refs                  map[TableName]*DataRow // contains references to other objects, ex.: hosts from the services table
	LastUpdate            int64                  // timestamp when this row has been updated
	dataString            []string               // stores string data
	dataInt               []int                  // stores integers
	dataInt64             []int64                // stores large integers
	dataFloat             []float64              // stores floats
	dataStringList        [][]string             // stores stringlists
	dataInt64List         [][]int64              // stores lists of integers
	dataServiceMemberList [][]ServiceMember      // stores list of servicemembers
	dataStringLarge       []StringContainer      // stores large strings
	dataInterfaceList     [][]interface{}
}

// NewDataRow creates a new DataRow
func NewDataRow(store *DataStore, raw *[]interface{}, columns *ColumnList, timestamp int64) (d *DataRow, err error) {
	d = &DataRow{
		LastUpdate: timestamp,
		DataStore:  store,
	}
	if raw == nil {
		// virtual tables without data have no references or ids
		return
	}

	if !store.Table.PassthroughOnly && len(store.Table.RefTables) > 0 {
		d.Refs = make(map[TableName]*DataRow, len(store.Table.RefTables))
	}

	err = d.SetData(raw, columns, timestamp)
	if err != nil {
		return
	}

	err = d.setReferences(store)
	return
}

// GetID calculates and the ID value
func (d *DataRow) GetID() string {
	if len(d.DataStore.Table.PrimaryKey) == 0 {
		return ""
	}
	if len(d.DataStore.Table.PrimaryKey) == 1 {
		id := d.GetStringByName(d.DataStore.Table.PrimaryKey[0])
		if *id == "" {
			log.Errorf("[%s] id for %s is null", d.DataStore.Peer.Name, d.DataStore.Table.Name.String())
		}
		return *id
	}

	var key strings.Builder
	for i, k := range d.DataStore.Table.PrimaryKey {
		if i > 0 {
			key.WriteString(ListSepChar1)
		}
		key.WriteString(*(d.GetStringByName(k)))
	}
	id := key.String()
	if id == "" || id == ListSepChar1 {
		log.Errorf("[%s] id for %s is null", d.DataStore.Peer.Name, d.DataStore.Table.Name.String())
	}
	return id
}

// GetID2 calculates and the ID value
func (d *DataRow) GetID2() (string, string) {
	id1 := d.GetStringByName(d.DataStore.Table.PrimaryKey[0])
	if *id1 == "" {
		log.Errorf("[%s] id1 for %s is null", d.DataStore.Peer.Name, d.DataStore.Table.Name.String())
	}
	id2 := d.GetStringByName(d.DataStore.Table.PrimaryKey[1])
	if *id2 == "" {
		log.Errorf("[%s] id2 for %s is null", d.DataStore.Peer.Name, d.DataStore.Table.Name.String())
	}
	return *id1, *id2
}

// setData creates initial data
func (d *DataRow) SetData(raw *[]interface{}, columns *ColumnList, timestamp int64) error {
	d.dataString = make([]string, d.DataStore.DataSizes[StringCol])
	d.dataStringList = make([][]string, d.DataStore.DataSizes[StringListCol])
	d.dataInt = make([]int, d.DataStore.DataSizes[IntCol])
	d.dataInt64 = make([]int64, d.DataStore.DataSizes[Int64Col])
	d.dataInt64List = make([][]int64, d.DataStore.DataSizes[Int64ListCol])
	d.dataFloat = make([]float64, d.DataStore.DataSizes[FloatCol])
	d.dataServiceMemberList = make([][]ServiceMember, d.DataStore.DataSizes[ServiceMemberListCol])
	d.dataInterfaceList = make([][]interface{}, d.DataStore.DataSizes[InterfaceListCol])
	d.dataStringLarge = make([]StringContainer, d.DataStore.DataSizes[StringLargeCol])
	return d.UpdateValues(0, raw, columns, timestamp)
}

// setReferences creates reference entries for cross referenced objects
func (d *DataRow) setReferences(store *DataStore) (err error) {
	for i := range store.Table.RefTables {
		ref := store.Table.RefTables[i]
		tableName := ref.Table.Name
		refsByName := store.Peer.Tables[tableName].Index
		refsByName2 := store.Peer.Tables[tableName].Index2

		switch len(ref.Columns) {
		case 1:
			d.Refs[tableName] = refsByName[*(d.GetString(ref.Columns[0]))]
		case 2:
			d.Refs[tableName] = refsByName2[*(d.GetString(ref.Columns[0]))][*(d.GetString(ref.Columns[1]))]
		}
		if d.Refs[tableName] == nil {
			if tableName == TableServices && (store.Table.Name == TableComments || store.Table.Name == TableDowntimes) {
				// this may happen for optional reference columns, ex. services in comments
				continue
			}
			return fmt.Errorf("%s reference not found from table %s, refmap contains %d elements", tableName.String(), store.Table.Name.String(), len(refsByName))
		}
	}
	return
}

// GetColumn returns the column by name
func (d *DataRow) GetColumn(name string) *Column {
	return d.DataStore.Table.ColumnsIndex[name]
}

// GetString returns the string value for given column
func (d *DataRow) GetString(col *Column) *string {
	switch col.StorageType {
	case LocalStore:
		switch col.DataType {
		case StringCol:
			return &(d.dataString[col.Index])
		case IntCol:
			val := fmt.Sprintf("%d", d.dataInt[col.Index])
			return &val
		case Int64Col:
			val := strconv.FormatInt(d.dataInt64[col.Index], 10)
			return &val
		case FloatCol:
			val := fmt.Sprintf("%v", d.dataFloat[col.Index])
			return &val
		case StringLargeCol:
			return d.dataStringLarge[col.Index].StringRef()
		case StringListCol:
			return joinStringlist(&d.dataStringList[col.Index], ListSepChar1)
		case ServiceMemberListCol:
			val := fmt.Sprintf("%v", d.dataServiceMemberList[col.Index])
			return &val
		case InterfaceListCol:
			val := fmt.Sprintf("%v", d.dataInterfaceList[col.Index])
			return &val
		case Int64ListCol:
			val := strings.Join(strings.Fields(fmt.Sprint(d.dataInt64List[col.Index])), ListSepChar1)
			return &val
		}
		log.Panicf("unsupported type: %s", col.DataType)
	case RefStore:
		ref := d.Refs[col.RefCol.Table.Name]
		if ref == nil {
			return interface2stringNoDedup(col.GetEmptyValue())
		}
		return ref.GetString(col.RefCol)
	}
	return interface2stringNoDedup(d.getVirtRowValue(col))
}

// GetStringByName returns the string value for given column name
func (d *DataRow) GetStringByName(name string) *string {
	return d.GetString(d.DataStore.Table.ColumnsIndex[name])
}

// GetStringList returns the string list for given column
func (d *DataRow) GetStringList(col *Column) *[]string {
	switch col.StorageType {
	case LocalStore:
		if col.DataType == StringListCol {
			return &(d.dataStringList[col.Index])
		}
		log.Panicf("unsupported type: %s", col.DataType)
	case RefStore:
		ref := d.Refs[col.RefCol.Table.Name]
		if ref == nil {
			return interface2stringlist(col.GetEmptyValue())
		}
		return ref.GetStringList(col.RefCol)
	}
	return interface2stringlist(d.getVirtRowValue(col))
}

// GetStringListByName returns the string list for given column name
func (d *DataRow) GetStringListByName(name string) *[]string {
	return d.GetStringList(d.DataStore.Table.ColumnsIndex[name])
}

// GetFloat returns the float64 value for given column
func (d *DataRow) GetFloat(col *Column) float64 {
	switch col.StorageType {
	case LocalStore:
		switch col.DataType {
		case FloatCol:
			return d.dataFloat[col.Index]
		case IntCol:
			return float64(d.dataInt[col.Index])
		case Int64Col:
			return float64(d.dataInt64[col.Index])
		}
		log.Panicf("unsupported type: %s", col.DataType)
	case RefStore:
		ref := d.Refs[col.RefCol.Table.Name]
		if ref == nil {
			return interface2float64(col.GetEmptyValue())
		}
		return ref.GetFloat(col.RefCol)
	}
	return interface2float64(d.getVirtRowValue(col))
}

// GetInt returns the int value for given column
func (d *DataRow) GetInt(col *Column) int {
	switch col.StorageType {
	case LocalStore:
		switch col.DataType {
		case IntCol:
			return d.dataInt[col.Index]
		case FloatCol:
			return int(d.dataFloat[col.Index])
		}
		log.Panicf("unsupported type: %s", col.DataType)
	case RefStore:
		ref := d.Refs[col.RefCol.Table.Name]
		if ref == nil {
			return interface2int(col.GetEmptyValue())
		}
		return ref.GetInt(col.RefCol)
	}
	return interface2int(d.getVirtRowValue(col))
}

// GetInt returns the int64 value for given column
func (d *DataRow) GetInt64(col *Column) int64 {
	switch col.StorageType {
	case LocalStore:
		switch col.DataType {
		case Int64Col:
			return d.dataInt64[col.Index]
		case IntCol:
			return int64(d.dataInt[col.Index])
		case FloatCol:
			return int64(d.dataFloat[col.Index])
		}
		log.Panicf("unsupported type: %s", col.DataType)
	case RefStore:
		ref := d.Refs[col.RefCol.Table.Name]
		if ref == nil {
			return interface2int64(col.GetEmptyValue())
		}
		return ref.GetInt64(col.RefCol)
	}
	return interface2int64(d.getVirtRowValue(col))
}

// GetIntByName returns the int value for given column name
func (d *DataRow) GetIntByName(name string) int {
	return d.GetInt(d.DataStore.Table.ColumnsIndex[name])
}

// GetInt64ByName returns the int value for given column name
func (d *DataRow) GetInt64ByName(name string) int64 {
	return d.GetInt64(d.DataStore.Table.ColumnsIndex[name])
}

// GetInt64List returns the int64 list for given column
func (d *DataRow) GetInt64List(col *Column) []int64 {
	switch col.StorageType {
	case LocalStore:
		if col.DataType == Int64ListCol {
			return d.dataInt64List[col.Index]
		}
		log.Panicf("unsupported type: %s", col.DataType)
	case RefStore:
		ref := d.Refs[col.RefCol.Table.Name]
		if ref == nil {
			return interface2int64list(col.GetEmptyValue())
		}
		return ref.GetInt64List(col.RefCol)
	}
	return interface2int64list(d.getVirtRowValue(col))
}

// GetHashMap returns the hashmap for given column
func (d *DataRow) GetHashMap(col *Column) map[string]string {
	switch col.StorageType {
	case LocalStore:
		log.Panicf("unsupported type: %s", col.DataType)
	case RefStore:
		ref := d.Refs[col.RefCol.Table.Name]
		if ref == nil {
			return interface2hashmap(col.GetEmptyValue())
		}
		return ref.GetHashMap(col.RefCol)
	}
	return interface2hashmap(d.getVirtRowValue(col))
}

// GetServiceMemberList returns the a list of service members
func (d *DataRow) GetServiceMemberList(col *Column) *[]ServiceMember {
	switch col.StorageType {
	case LocalStore:
		if col.DataType == ServiceMemberListCol {
			return &(d.dataServiceMemberList[col.Index])
		}
		log.Panicf("unsupported type: %s", col.DataType)
	case RefStore:
		ref := d.Refs[col.RefCol.Table.Name]
		if ref == nil {
			return interface2servicememberlist(col.GetEmptyValue())
		}
		return ref.GetServiceMemberList(col.RefCol)
	}
	return interface2servicememberlist(d.getVirtRowValue(col))
}

// GetInterfaceList returns the a list of interfaces
func (d *DataRow) GetInterfaceList(col *Column) []interface{} {
	switch col.StorageType {
	case LocalStore:
		if col.DataType == InterfaceListCol {
			return d.dataInterfaceList[col.Index]
		}
		log.Panicf("unsupported type: %s", col.DataType)
	case RefStore:
		ref := d.Refs[col.RefCol.Table.Name]
		if ref == nil {
			return interface2interfacelist(col.GetEmptyValue())
		}
		return ref.GetInterfaceList(col.RefCol)
	}
	return interface2interfacelist(d.getVirtRowValue(col))
}

// GetValueByColumn returns the raw value for given column
func (d *DataRow) GetValueByColumn(col *Column) interface{} {
	if col.Optional != NoFlags && !d.DataStore.Peer.HasFlag(col.Optional) {
		return col.GetEmptyValue()
	}
	switch col.StorageType {
	case LocalStore:
		switch col.DataType {
		case StringCol:
			return d.dataString[col.Index]
		case StringListCol:
			return d.dataStringList[col.Index]
		case StringLargeCol:
			return d.dataStringLarge[col.Index].StringRef()
		case IntCol:
			return d.dataInt[col.Index]
		case Int64ListCol:
			return d.dataInt64List[col.Index]
		case Int64Col:
			return d.dataInt64[col.Index]
		case FloatCol:
			return d.dataFloat[col.Index]
		default:
			log.Panicf("unsupported column %s (type %d) in table %s", col.Name, col.DataType, d.DataStore.Table.Name)
		}
	case RefStore:
		return d.Refs[col.RefCol.Table.Name].GetValueByColumn(col.RefCol)
	case VirtStore:
		return d.getVirtRowValue(col)
	default:
		log.Panicf("unsupported column %s (type %d) in table %s", col.Name, col.DataType, d.DataStore.Table.Name)
	}
	return ""
}

// getVirtRowValue returns the actual value for a virtual column.
func (d *DataRow) getVirtRowValue(col *Column) interface{} {
	var value interface{}
	if col.VirtMap.StatusKey != "" {
		if d.DataStore.Peer == nil {
			log.Panicf("requesting column '%s' from table '%s' with peer", col.Name, d.DataStore.Table.Name)
		}
		p := d.DataStore.Peer
		ok := false
		if p.HasFlag(LMDSub) {
			value, ok = d.getVirtSubLMDValue(col)
		}
		if !ok {
			value = p.StatusGet(col.VirtMap.StatusKey)
		}
	} else {
		value = col.VirtMap.ResolvFunc(d, col)
	}
	return cast2Type(value, col)
}

// VirtColStateOrder returns sortable state
func VirtColLastStateChangeOrder(d *DataRow, col *Column) interface{} {
	// return last_state_change or program_start
	lastStateChange := d.GetIntByName("last_state_change")
	if lastStateChange == 0 {
		return d.DataStore.Peer.Status["ProgramStart"]
	}
	return lastStateChange
}

// VirtColStateOrder returns sortable state
func VirtColStateOrder(d *DataRow, col *Column) interface{} {
	// return 4 instead of 2, which makes critical come first
	// this way we can use this column to sort by state
	state := d.GetIntByName("state")
	if state == 2 {
		return 4
	}
	return state
}

// VirtColHasLongPluginOutput returns 1 if there is long plugin output, 0 if not
func VirtColHasLongPluginOutput(d *DataRow, col *Column) interface{} {
	val := d.GetStringByName("long_plugin_output")
	if *val != "" {
		return 1
	}
	return 0
}

// VirtColServicesWithInfo returns list of services with additional information
func VirtColServicesWithInfo(d *DataRow, col *Column) interface{} {
	services := d.GetStringListByName("services")
	hostName := d.GetStringByName("name")
	servicesStore := d.DataStore.Peer.Tables[TableServices]
	stateCol := servicesStore.Table.GetColumn("state")
	checkedCol := servicesStore.Table.GetColumn("has_been_checked")
	outputCol := servicesStore.Table.GetColumn("plugin_output")
	res := make([]interface{}, 0)
	for i := range *services {
		service, ok := servicesStore.Index2[*hostName][(*services)[i]]
		if !ok {
			log.Errorf("Could not find service: %s - %s\n", *hostName, (*services)[i])
			continue
		}
		serviceValue := []interface{}{(*services)[i], service.GetInt(stateCol), service.GetInt(checkedCol)}
		if col.Name == "services_with_info" {
			serviceValue = append(serviceValue, service.GetString(outputCol))
		}
		res = append(res, serviceValue)
	}
	return res
}

// VirtColMembersWithState returns a list of hostgroup/servicegroup members with their states
func VirtColMembersWithState(d *DataRow, col *Column) interface{} {
	res := make([]interface{}, 0)
	switch d.DataStore.Table.Name {
	case TableHostgroups:
		members := d.GetStringListByName("members")
		hostsStore := d.DataStore.Peer.Tables[TableHosts]
		stateCol := hostsStore.Table.GetColumn("state")
		checkedCol := hostsStore.Table.GetColumn("has_been_checked")

		for _, hostName := range *members {
			host, ok := hostsStore.Index[hostName]
			if !ok {
				log.Errorf("Could not find host: %s\n", hostName)
				continue
			}
			hostValue := []interface{}{hostName, host.GetInt(stateCol), host.GetInt(checkedCol)}
			res = append(res, hostValue)
		}
	case TableServicegroups:
		membersCol := d.DataStore.GetColumn("members")
		members := d.GetServiceMemberList(membersCol)
		servicesStore := d.DataStore.Peer.Tables[TableServices]
		stateCol := servicesStore.Table.GetColumn("state")
		checkedCol := servicesStore.Table.GetColumn("has_been_checked")

		for i := range *members {
			hostName := (*members)[i][0]
			serviceDescription := (*members)[i][1]

			service, ok := servicesStore.Index2[hostName][serviceDescription]
			if !ok {
				log.Errorf("Could not find service: %s - %s\n", hostName, serviceDescription)
				continue
			}
			serviceValue := []interface{}{hostName, serviceDescription, service.GetInt(stateCol), service.GetInt(checkedCol)}
			res = append(res, serviceValue)
		}
	}
	return res
}

// VirtColComments returns list of comment IDs (with optional additional information)
func VirtColComments(d *DataRow, col *Column) interface{} {
	commentsStore := d.DataStore.Peer.Tables[TableComments]
	commentsTable := commentsStore.Table
	authorCol := commentsTable.GetColumn("author")
	commentCol := commentsTable.GetColumn("comment")
	res := make([]interface{}, 0)
	comments, ok := d.DataStore.Peer.CommentsCache[d]
	if !ok {
		return res
	}
	if col.Name == "comments" {
		return comments
	}
	for i := range comments {
		commentID := fmt.Sprintf("%d", comments[i])
		comment, ok := commentsStore.Index[commentID]
		if !ok {
			log.Errorf("Could not find comment: %s\n", commentID)
			continue
		}
		commentWithInfo := []interface{}{comments[i], comment.GetString(authorCol), comment.GetString(commentCol)}
		res = append(res, commentWithInfo)
	}
	return res
}

// VirtColDowntimes returns list of downtimes IDs
func VirtColDowntimes(d *DataRow, col *Column) interface{} {
	downtimesStore := d.DataStore.Peer.Tables[TableDowntimes]
	downtimesTable := downtimesStore.Table
	authorCol := downtimesTable.GetColumn("author")
	commentCol := downtimesTable.GetColumn("comment")
	res := make([]interface{}, 0)
	downtimes, ok := d.DataStore.Peer.DowntimesCache[d]
	if !ok {
		return res
	}
	if col.Name == "downtimes" {
		return downtimes
	}
	for i := range downtimes {
		downtimeID := fmt.Sprintf("%d", downtimes[i])
		downtime, ok := downtimesStore.Index[downtimeID]
		if !ok {
			log.Errorf("Could not find downtime: %s\n", downtimeID)
			continue
		}
		downtimeWithInfo := []interface{}{downtimes[i], downtime.GetString(authorCol), downtime.GetString(commentCol)}
		res = append(res, downtimeWithInfo)
	}
	return res
}

// VirtColCustomVariables returns a custom variables hash
func VirtColCustomVariables(d *DataRow, col *Column) interface{} {
	namesCol := d.DataStore.Table.GetColumn("custom_variable_names")
	valuesCol := d.DataStore.Table.GetColumn("custom_variable_values")
	names := d.GetStringList(namesCol)
	values := d.GetStringList(valuesCol)
	res := make(map[string]string, len(*names))
	for i := range *names {
		res[(*names)[i]] = (*values)[i]
	}
	return res
}

// VirtColHasLongPluginOutput returns 1 if there is long plugin output, 0 if not
func VirtColTotalServices(d *DataRow, col *Column) interface{} {
	val := d.GetIntByName("num_services")
	return val
}

// getVirtSubLMDValue returns status values for LMDSub backends
func (d *DataRow) getVirtSubLMDValue(col *Column) (val interface{}, ok bool) {
	ok = true
	p := d.DataStore.Peer
	peerData := p.StatusGet("SubPeerStatus").(map[string]interface{})
	if peerData == nil {
		return nil, false
	}
	switch col.Name {
	case "status":
		// return worst state of LMD and LMDSubs state
		parentVal := p.StatusGet("PeerStatus").(PeerStatus)
		if parentVal != PeerStatusUp {
			val = parentVal
		} else {
			val, ok = peerData[col.Name]
		}
	case "last_error":
		// return worst state of LMD and LMDSubs state
		parentVal := p.StatusGet("LastError").(string)
		val, ok = peerData[col.Name]
		if parentVal != "" && (!ok || val.(string) == "") {
			val = parentVal
		}
	default:
		val, ok = peerData[col.Name]
	}
	return
}

// MatchFilter returns true if the given filter matches the given datarow.
func (d *DataRow) MatchFilter(filter *Filter) bool {
	// recursive group filter
	filterLength := len(filter.Filter)
	if filterLength > 0 {
		for i := range filter.Filter {
			f := filter.Filter[i]
			subresult := d.MatchFilter(f)
			switch filter.GroupOperator {
			case And:
				// if all conditions must match and we failed already, exit early
				if !subresult {
					return false
				}
			case Or:
				// if only one condition must match and we got that already, exit early
				if subresult {
					return true
				}
			}
		}
		// if this is an AND filter and we did not return yet, this means all have matched.
		// else its an OR filter and none has matched so far.
		return filter.GroupOperator == And
	}

	// if this is a optional column and we do not meet the requirements, match against an empty default column
	if filter.Column.Optional != NoFlags && !d.DataStore.Peer.HasFlag(filter.Column.Optional) {
		// duplicate filter, but use the empty column
		f := &Filter{
			Column:    d.DataStore.Table.GetEmptyColumn(),
			Operator:  filter.Operator,
			StrValue:  filter.StrValue,
			Regexp:    filter.Regexp,
			IsEmpty:   filter.IsEmpty,
			CustomTag: filter.CustomTag,
			Negate:    filter.Negate,
		}
		if f.Negate {
			return !f.Match(d)
		}
		return f.Match(d)
	}
	if filter.Negate {
		return !filter.Match(d)
	}
	return filter.Match(d)
}

func (d *DataRow) getStatsKey(res *Response) string {
	if len(res.Request.RequestColumns) == 0 {
		return ""
	}
	keyValues := []string{}
	for i := range res.Request.RequestColumns {
		keyValues = append(keyValues, *(d.GetString(res.Request.RequestColumns[i])))
	}
	return strings.Join(keyValues, ListSepChar1)
}

// UpdateValues updates this datarow with new values
func (d *DataRow) UpdateValues(dataOffset int, data *[]interface{}, columns *ColumnList, timestamp int64) error {
	if len(*columns) != len(*data)-dataOffset {
		return fmt.Errorf("table %s update failed, data size mismatch, expected %d columns and got %d", d.DataStore.Table.Name.String(), len(*columns), len(*data))
	}
	for i := range *columns {
		col := (*columns)[i]
		i += dataOffset
		switch col.DataType {
		case StringCol:
			d.dataString[col.Index] = *(interface2string((*data)[i]))
		case StringListCol:
			if col.FetchType == Static {
				// deduplicate string lists
				d.dataStringList[col.Index] = *d.deduplicateStringlist(interface2stringlist((*data)[i]))
			} else {
				d.dataStringList[col.Index] = *interface2stringlist((*data)[i])
			}
		case StringLargeCol:
			d.dataStringLarge[col.Index] = *interface2stringlarge((*data)[i])
		case IntCol:
			d.dataInt[col.Index] = interface2int((*data)[i])
		case Int64Col:
			d.dataInt64[col.Index] = interface2int64((*data)[i])
		case Int64ListCol:
			d.dataInt64List[col.Index] = interface2int64list((*data)[i])
		case FloatCol:
			d.dataFloat[col.Index] = interface2float64((*data)[i])
		case ServiceMemberListCol:
			d.dataServiceMemberList[col.Index] = *interface2servicememberlist((*data)[i])
		case InterfaceListCol:
			d.dataInterfaceList[col.Index] = interface2interfacelist((*data)[i])
		default:
			log.Panicf("unsupported column %s (type %d) in table %s", col.Name, col.DataType, d.DataStore.Table.Name)
		}
	}
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	d.LastUpdate = timestamp
	return nil
}

// UpdateValuesNumberOnly updates this datarow with new values but skips strings
func (d *DataRow) UpdateValuesNumberOnly(dataOffset int, data *[]interface{}, columns *ColumnList, timestamp int64) error {
	if len(*columns) != len(*data)-dataOffset {
		return fmt.Errorf("table %s update failed, data size mismatch, expected %d columns and got %d", d.DataStore.Table.Name.String(), len(*columns), len(*data))
	}
	for i := range *columns {
		col := (*columns)[i]
		i += dataOffset
		switch col.DataType {
		case IntCol:
			d.dataInt[col.Index] = interface2int((*data)[i])
		case Int64Col:
			d.dataInt64[col.Index] = interface2int64((*data)[i])
		case Int64ListCol:
			d.dataInt64List[col.Index] = interface2int64list((*data)[i])
		case FloatCol:
			d.dataFloat[col.Index] = interface2float64((*data)[i])
		case InterfaceListCol:
			d.dataInterfaceList[col.Index] = interface2interfacelist((*data)[i])
		}
	}
	d.LastUpdate = timestamp
	return nil
}

// CheckChangedIntValues returns true if the given data results in an update
func (d *DataRow) CheckChangedIntValues(data *[]interface{}, columns *ColumnList) bool {
	for j := range *columns {
		if interface2int((*data)[j]) != d.dataInt[(*columns)[j].Index] {
			return true
		}
	}
	return false
}

func interface2float64(in interface{}) float64 {
	switch v := in.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case bool:
		if v {
			return 1
		}
	case string:
		val, _ := strconv.ParseFloat(v, 64)
		return val
	default:
		val, _ := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
		return val
	}
	return 0
}

func interface2int(in interface{}) int {
	switch v := in.(type) {
	case float64:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	case int32:
		return int(v)
	case bool:
		if v {
			return 1
		}
		return 0
	case string:
		val, _ := strconv.ParseInt(v, 10, 64)
		return int(val)
	}
	val, _ := strconv.ParseInt(fmt.Sprintf("%v", in), 10, 64)
	return int(val)
}

func interface2int64(in interface{}) int64 {
	switch v := in.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case bool:
		if v {
			return 1
		}
	case string:
		val, _ := strconv.ParseInt(v, 10, 64)
		return val
	}
	val, _ := strconv.ParseInt(fmt.Sprintf("%v", in), 10, 64)
	return val
}

func interface2string(in interface{}) *string {
	switch v := in.(type) {
	case string:
		dedupedstring := stringdedup.S(v)
		return &dedupedstring
	case *string:
		return v
	case nil:
		val := ""
		return &val
	}
	str := fmt.Sprintf("%v", in)
	return &str
}

func interface2stringNoDedup(in interface{}) *string {
	switch v := in.(type) {
	case string:
		return &v
	case *string:
		return v
	case nil:
		val := ""
		return &val
	}
	str := fmt.Sprintf("%v", in)
	return &str
}

func interface2stringlarge(in interface{}) *StringContainer {
	switch v := in.(type) {
	case string:
		return NewStringContainer(&v)
	case *string:
		return NewStringContainer(v)
	case nil:
		val := ""
		return NewStringContainer(&val)
	case *StringContainer:
		return v
	}
	str := fmt.Sprintf("%v", in)
	return NewStringContainer(&str)
}

func interface2stringlist(in interface{}) *[]string {
	switch list := in.(type) {
	case *[]string:
		return list
	case []string:
		return &list
	case float64:
		val := make([]string, 0, 1)
		// icinga 2 sends a 0 for empty lists, ex.: modified_attributes_list
		if in != 0 {
			val = append(val, *(interface2string(in)))
		}
		return &val
	case []interface{}:
		val := make([]string, 0, len(list))
		for i := range list {
			val = append(val, *(interface2string(list[i])))
		}
		return &val
	}
	log.Warnf("unsupported string list type: %v", in)
	val := make([]string, 0)
	return &val
}

func interface2servicememberlist(in interface{}) *[]ServiceMember {
	switch list := in.(type) {
	case *[]ServiceMember:
		return list
	case []ServiceMember:
		return &list
	case []interface{}:
		val := make([]ServiceMember, len(list))
		for i := range list {
			if l2, ok := list[i].([]interface{}); ok {
				if len(l2) == 2 {
					val[i][0] = *interface2string(l2[0])
					val[i][1] = *interface2string(l2[1])
				}
			}
		}
		return &val
	}
	log.Warnf("unsupported service list type: %v", in)
	val := make([]ServiceMember, 0)
	return &val
}

func interface2int64list(in interface{}) []int64 {
	if list, ok := in.([]int64); ok {
		return (list)
	}
	if in == nil {
		val := make([]int64, 0)
		return val
	}
	if list, ok := in.([]int); ok {
		val := make([]int64, 0, len(list))
		for i := range list {
			val = append(val, interface2int64(list[i]))
		}
		return val
	}
	if list, ok := in.([]interface{}); ok {
		val := make([]int64, 0, len(list))
		for i := range list {
			val = append(val, interface2int64(list[i]))
		}
		return val
	}
	log.Warnf("unsupported int list type: %v", in)
	val := make([]int64, 0)
	return val
}

// interface2hashmap converts an interface to a hashmap
func interface2hashmap(in interface{}) map[string]string {
	if in == nil {
		val := make(map[string]string)
		return val
	}

	switch list := in.(type) {
	case map[string]string:
		return list
	case []interface{}:
		val := make(map[string]string)
		for _, tupelInterface := range list {
			if tupel, ok := tupelInterface.([]interface{}); ok {
				if len(tupel) == 2 {
					if s, ok := tupel[1].(string); ok {
						val[tupel[0].(string)] = s
					} else {
						val[tupel[0].(string)] = fmt.Sprintf("%v", tupel[1])
					}
				}
			}
		}
		return val
	case map[string]interface{}:
		val := make(map[string]string)
		for k, v := range list {
			if s, ok := v.(string); ok {
				val[k] = s
			} else {
				val[k] = ""
			}
		}
		return val
	default:
		log.Warnf("unsupported hashmap list type: %v", in)
		val := make(map[string]string)
		return val
	}
}

// interface2interfacelist converts anything to a list of interfaces
func interface2interfacelist(in interface{}) []interface{} {
	if list, ok := in.([]interface{}); ok {
		return (list)
	}
	log.Warnf("unsupported interface list type: %v", in)
	val := make([]interface{}, 0)
	return val
}

// deduplicateStringlist store duplicate string lists only once
func (d *DataRow) deduplicateStringlist(list *[]string) *[]string {
	sum := sha256.Sum256([]byte(*joinStringlist(list, ListSepChar1)))
	if l, ok := d.DataStore.dupStringList[sum]; ok {
		return &l
	}
	d.DataStore.dupStringList[sum] = *list
	return list
}

// joinStringlist joins list with given character
func joinStringlist(list *[]string, join string) *string {
	var joined strings.Builder
	for _, s := range *list {
		joined.WriteString(s)
		joined.WriteString(join)
	}
	str := joined.String()
	return &str
}

func cast2Type(val interface{}, col *Column) interface{} {
	switch col.DataType {
	case StringCol:
		return (interface2stringNoDedup(val))
	case StringListCol:
		return (interface2stringlist(val))
	case IntCol:
		return (interface2int(val))
	case Int64Col:
		return (interface2int64(val))
	case Int64ListCol:
		return (interface2int64list(val))
	case FloatCol:
		return (interface2float64(val))
	case HashMapCol, CustomVarCol:
		return (interface2hashmap(val))
	case ServiceMemberListCol:
		return (interface2servicememberlist(val))
	case InterfaceListCol:
		return val
	}
	log.Panicf("unsupported type: %s", col.DataType)
	return nil
}

// WriteJSON store duplicate string lists only once
func (d *DataRow) WriteJSON(json *jsoniter.Stream, columns *[]*Column) {
	json.WriteRaw("[")
	for i := range *columns {
		col := (*columns)[i]
		if i > 0 {
			json.WriteRaw(",")
		}
		if col.Optional != NoFlags && !d.DataStore.Peer.HasFlag(col.Optional) {
			json.WriteVal(col.GetEmptyValue())
			continue
		}
		switch col.DataType {
		case StringCol, StringLargeCol:
			json.WriteString(*(d.GetString(col)))
		case StringListCol:
			json.WriteVal(d.GetStringList(col))
		case IntCol:
			json.WriteInt(d.GetInt(col))
		case Int64Col:
			json.WriteInt64(d.GetInt64(col))
		case FloatCol:
			json.WriteFloat64(d.GetFloat(col))
		case Int64ListCol:
			json.WriteVal(d.GetInt64List(col))
		case ServiceMemberListCol:
			json.WriteVal(d.GetServiceMemberList(col))
		case InterfaceListCol:
			json.WriteVal(d.GetInterfaceList(col))
		case CustomVarCol:
			fallthrough
		case HashMapCol:
			json.WriteVal(d.GetHashMap(col))
		default:
			log.Panicf("unsupported type: %s", col.DataType)
		}
	}
	json.WriteRaw("]\n")
}

func (d *DataRow) isAuthorizedFor(authUser string, host string, service string) (canView bool) {
	canView = false

	p := d.DataStore.Peer

	// get contacts for host, if we are checking a host or
	// if this is a service and ServiceAuthorization is loose
	if (service != "" && p.LocalConfig.ServiceAuthorization == AuthLoose) || service == "" {
		hostObj, ok := p.Tables[TableHosts].Index[host]
		contactsColumn := p.Tables[TableHosts].GetColumn("contacts")
		// Make sure the host we found is actually valid
		if !ok {
			return
		}
		for _, contact := range *hostObj.GetStringList(contactsColumn) {
			if contact == authUser {
				canView = true
				return
			}
		}
	}

	// get contacts on services
	if service != "" {
		serviceObj, ok := p.Tables[TableServices].Index2[host][service]
		contactsColumn := p.Tables[TableServices].GetColumn("contacts")
		if !ok {
			return
		}
		for _, contact := range *serviceObj.GetStringList(contactsColumn) {
			if contact == authUser {
				canView = true
				return
			}
		}
	}

	return
}

func (d *DataRow) isAuthorizedForHostGroup(authUser string, hostgroup string) (canView bool) {
	p := d.DataStore.Peer
	canView = false

	hostgroupObj, ok := p.Tables[TableHostgroups].Index[hostgroup]
	membersColumn := p.Tables[TableHostgroups].GetColumn("members")
	if !ok {
		return
	}

	members := *(*hostgroupObj).GetStringList(membersColumn)
	for i, hostname := range members {
		/* If GroupAuthorization is loose, we just need to find the contact
		 * in any hosts in the group, then we can return.
		 * If it is strict, the user must be a contact on every single host.
		 * Therefore we return false if a contact doesn't exists on a host
		 * and then on the last iteration return true if the contact is a contact
		 * on the final host
		 */
		switch p.LocalConfig.GroupAuthorization {
		case AuthLoose:
			if d.isAuthorizedFor(authUser, hostname, "") {
				canView = true
				return
			}
		case AuthStrict:
			if !d.isAuthorizedFor(authUser, hostname, "") {
				canView = false
				return
			} else if i == len(members)-1 {
				canView = true
				return
			}
		}
	}

	return
}

func (d *DataRow) isAuthorizedForServiceGroup(authUser string, servicegroup string) (canView bool) {
	p := d.DataStore.Peer
	canView = false

	servicegroupObj, ok := p.Tables[TableServicegroups].Index[servicegroup]
	membersColumn := p.Tables[TableServicegroups].GetColumn("members")
	if !ok {
		return
	}

	members := *((*servicegroupObj).GetServiceMemberList(membersColumn))
	for i := range members {
		/* If GroupAuthorization is loose, we just need to find the contact
		 * in any hosts in the group, then we can return.
		 * If it is strict, the user must be a contact on every single host.
		 * Therefore we return false if a contact doesn't exists on a host
		 * and then on the last iteration return true if the contact is a contact
		 * on the final host
		 */
		switch p.LocalConfig.GroupAuthorization {
		case AuthLoose:
			if d.isAuthorizedFor(authUser, members[i][0], members[i][1]) {
				canView = true
				return
			}
		case AuthStrict:
			if !d.isAuthorizedFor(authUser, members[i][0], members[i][1]) {
				canView = false
				return
			} else if i == len(members)-1 {
				canView = true
				return
			}
		}
	}

	return
}

func (d *DataRow) checkAuth(authUser string) (canView bool) {
	// Return if no AuthUser is set, or the table does not support AuthUser
	if authUser == "" {
		canView = true
		return
	}

	table := d.DataStore.Table

	switch table.Name {
	case TableHosts:
		hostNameIndex := table.GetColumn("name").Index
		hostName := d.dataString[hostNameIndex]
		canView = d.isAuthorizedFor(authUser, hostName, "")
	case TableServices:
		hostNameIndex := table.GetColumn("host_name").Index
		hostName := d.dataString[hostNameIndex]
		serviceIndex := table.GetColumn("description").Index
		serviceDescription := d.dataString[serviceIndex]
		canView = d.isAuthorizedFor(authUser, hostName, serviceDescription)
	case TableHostgroups:
		nameIndex := table.GetColumn("name").Index
		hostgroupName := d.dataString[nameIndex]
		canView = d.isAuthorizedForHostGroup(authUser, hostgroupName)
	case TableServicegroups:
		nameIndex := table.GetColumn("name").Index
		servicegroupName := d.dataString[nameIndex]
		canView = d.isAuthorizedForServiceGroup(authUser, servicegroupName)
	case TableHostsbygroup:
		hostNameIndex := table.GetColumn("name").Index
		hostName := d.dataString[hostNameIndex]
		hostGroupIndex := table.GetColumn("hostgroup_name").Index
		hostGroupName := d.dataString[hostGroupIndex]
		canView = d.isAuthorizedFor(authUser, hostName, "") &&
			d.isAuthorizedForHostGroup(authUser, hostGroupName)
	case TableServicesbygroup, TableServicesbyhostgroup:
		hostNameIndex := table.GetColumn("host_name").Index
		hostName := d.dataString[hostNameIndex]
		serviceIndex := table.GetColumn("description").Index
		serviceDescription := d.dataString[serviceIndex]

		if table.Name == TableServicesbygroup {
			servicegroupIndex := table.GetColumn("servicegroup_name").Index
			servicegroupName := d.dataString[servicegroupIndex]
			canView = d.isAuthorizedFor(authUser, hostName, serviceDescription) && d.isAuthorizedForServiceGroup(authUser, servicegroupName)
		} else {
			hostgroupIndex := table.GetColumn("hostgroup_name").Index
			hostgroupName := d.dataString[hostgroupIndex]
			canView = d.isAuthorizedFor(authUser, hostName, serviceDescription) && d.isAuthorizedForHostGroup(authUser, hostgroupName)
		}
	case TableDowntimes, TableComments:
		hostIndex := table.GetColumn("host_name").Index
		serviceIndex := table.GetColumn("service_description").Index
		hostName := d.dataString[hostIndex]
		serviceDescription := d.dataString[serviceIndex]
		canView = d.isAuthorizedFor(authUser, hostName, serviceDescription)
	default:
		canView = true
	}
	return
}
