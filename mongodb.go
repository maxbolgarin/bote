package bote

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/maxbolgarin/contem"
	"github.com/maxbolgarin/errm"
	"github.com/maxbolgarin/gorder"
	"github.com/maxbolgarin/logze"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
	// ErrNotFound is returned when a document is not found.
	ErrNotFound = errm.New("not found")
	// ErrDuplicate is returned when a document is already exists.
	ErrDuplicate = errm.New("duplicate")
)

// DatabaseConfig contains database configuration for creating MongoDB client.
//
// You can use environment variables to fill it:
// BOTE_DB_ADDRESS - MongoDB address
// BOTE_DB_NAME - database name
// BOTE_DB_USERNAME - MongoDB username
// BOTE_DB_PASSWORD - MongoDB password
// BOTE_DB_DISABLED - set to true if you don't want to use MongoDB
type DatabaseConfig struct {
	// Address is the MongoDB address in ip:port format.
	Address string `yaml:"address" envconfig:"BOTE_DB_ADDRESS"`
	// DBName is the name of the MongoDB database.
	DBName string `yaml:"db_name" envconfig:"BOTE_DB_NAME"`
	// Username is the MongoDB username.
	Username string `yaml:"username" envconfig:"BOTE_DB_USERNAME"`
	// Password is the MongoDB password.
	Password string `yaml:"password" envconfig:"BOTE_DB_PASSWORD"`

	// Disabled is a flag that disables the MongoDB client and use bot without persistent storage.
	Disabled bool `yaml:"disabled" envconfig:"BOTE_DB_DISABLED"`
}

// Validate validates database configuration.
func (cfg DatabaseConfig) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Address, validation.Required.When(!cfg.Disabled)),
		validation.Field(&cfg.DBName, validation.Required.When(!cfg.Disabled)),
		validation.Field(&cfg.Username, validation.Required.When(len(cfg.Password) > 0 && !cfg.Disabled)),
		validation.Field(&cfg.Password, validation.Required.When(len(cfg.Username) > 0 && !cfg.Disabled)),
	)
}

// MongoDB is a MongoDB client, that creates collections and handles transactions.
type MongoDB struct {
	database *mongo.Database
	client   *mongo.Client
	log      logze.Logger

	colls map[string]*Collection
	mu    sync.RWMutex
}

// NewMongo creates a new MongoDB client.
func NewMongo(ctx contem.Context, cfg DatabaseConfig) (*MongoDB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("mongodb://%s/%s", cfg.Address, cfg.DBName)
	opts := options.Client().ApplyURI(dsn)
	if len(cfg.Username) > 0 && len(cfg.Password) > 0 {
		opts.SetAuth(options.Credential{
			AuthMechanism: "SCRAM-SHA-256",
			AuthSource:    cfg.DBName,
			Username:      cfg.Username,
			Password:      cfg.Password,
		})
	}

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	ctx.Add(client.Disconnect)

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}

	db := &MongoDB{
		database: client.Database(cfg.DBName),
		client:   client,
		colls:    make(map[string]*Collection),
	}

	return db, err
}

// GetCollection returns a collection object by name.
// It will create a new collection if it doesn't exist after first query.
func (m *MongoDB) GetCollection(name string) *Collection {
	m.mu.RLock()
	coll, ok := m.colls[name]
	m.mu.RUnlock()

	if ok {
		return coll
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.colls[name] = &Collection{
		coll: m.database.Collection(name),
		name: name,
	}

	return m.colls[name]
}

// MakeTransaction executes a transaction.
// It will create a new session and execute a function inside a transaction.
func (m *MongoDB) MakeTransaction(ctx context.Context, fn func() (any, error)) (any, error) {
	session, err := m.client.StartSession()
	if err != nil {
		return nil, errm.Wrap(err, "start session")
	}
	defer session.EndSession(ctx)

	sessionFunc := func(sessionCtx mongo.SessionContext) (any, error) {
		return fn()
	}

	result, err := session.WithTransaction(ctx, sessionFunc)
	if err != nil {
		return nil, errm.Wrap(err, "transaction")
	}

	return result, nil
}

// Collection handles interactions with a MongoDB collection.
type Collection struct {
	coll *mongo.Collection
	name string
}

// CreateIndex creates an index for a collection with the given field names.
func (m *Collection) CreateIndex(ctx context.Context, fieldNames ...string) error {
	return m.createIndex(ctx, fieldNames, false)
}

// CreateUniqueIndex creates a unique index for a collection with the given field names.
func (m *Collection) CreateUniqueIndex(ctx context.Context, fieldNames ...string) error {
	return m.createIndex(ctx, fieldNames, true)
}

// FindOne finds a single document in the collection.
// Use filter to filter the document, e.g. {key: value}
func (m *Collection) FindOne(ctx context.Context, dest any, filter Filter) error {
	result := m.coll.FindOne(ctx, prepareFilter(filter))
	err := result.Err()

	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return ErrNotFound
	case err != nil:
		return err
	}

	if err := result.Decode(dest); err != nil {
		return errm.Wrap(err, "decode")
	}

	return nil
}

// FindIn finds multiple documents in the collection using in filter.
// Use filterIn to filter with $in operator, e.g. {key: {$in: [value1, value2, ...]}}
func (m *Collection) FindIn(ctx context.Context, dest any, filter Filter, filterIn Filter) error {
	return m.find(ctx, dest, mergeFilter(prepareFilter(filterIn, included), filter))
}

// FindFrom finds multiple documents in the collection using from filter.
// Use filterFrom to filter with $from operator, e.g. {key: {$from: value1}}
func (m *Collection) FindFrom(ctx context.Context, dest any, filter, filterFrom Filter) error {
	return m.find(ctx, dest, mergeFilter(prepareFilter(filterFrom, from), filter))
}

// FindMany finds all documents in the collection using filter.
func (m *Collection) FindMany(ctx context.Context, dest any, filter Filter) error {
	return m.find(ctx, dest, prepareFilter(filter))
}

// FindAll finds all documents in the collection.
func (m *Collection) FindAll(ctx context.Context, dest any) error {
	return m.find(ctx, dest, prepareFilter(nil))
}

// Count counts the number of documents in the collection using filter.
func (m *Collection) Count(ctx context.Context, filter Filter) (int64, error) {
	count, err := m.coll.CountDocuments(ctx, prepareFilter(filter))
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return 0, ErrNotFound
	case err != nil:
		return 0, err
	}
	return count, nil
}

// Insert inserts a document into the collection.
func (m *Collection) Insert(ctx context.Context, record any) error {
	_, err := m.coll.InsertOne(ctx, record)
	switch {
	case isDuplicateErr(err):
		return ErrDuplicate
	case err != nil:
		return err
	}
	return nil
}

// Replace replaces a document in the collection.
func (m *Collection) Replace(ctx context.Context, record any, filter Filter) error {
	trueUpsert := true
	_, err := m.coll.ReplaceOne(ctx, prepareFilter(filter), record, &options.ReplaceOptions{
		Upsert: &trueUpsert,
	})
	if err != nil {
		return err
	}
	return nil
}

// SetFields sets fields in a document in the collection using updates map.
// For example: {key1: value1, key2: value2} becomes {$set: {key1: value1, key2: value2}}
func (m *Collection) SetFields(ctx context.Context, filter Filter, update Updates) error {
	return m.updateOne(ctx, filter, prepareUpdate(set, update))
}

// SetFromDiff sets fields in a document in the collection using diff structure.
func (m *Collection) SetFromDiff(ctx context.Context, filter Filter, diff any) error {
	update, err := diffToUpdates(diff)
	if err != nil {
		return errm.Wrap(err, "diff to updates")
	}

	if err := m.updateOne(ctx, filter, update); err != nil {
		return err
	}
	return nil
}

// DeleteFields deletes fields in a document in the collection.
// For example: {key1, key2} becomes {$unset: {key1: "", key2: ""}}
func (m *Collection) DeleteFields(ctx context.Context, filter Filter, fields []string) error {
	updateInfo := make(map[string]any)
	for _, f := range fields {
		updateInfo[f] = ""
	}
	return m.updateOne(ctx, filter, prepareUpdate(unset, updateInfo))
}

// Delete deletes a document in the collection.
func (m *Collection) Delete(ctx context.Context, filter Filter) error {
	_, err := m.coll.DeleteOne(ctx, prepareFilter(filter))
	if err != nil {
		return err
	}
	return nil
}

func (m *Collection) createIndex(ctx context.Context, fieldNames []string, isUnique bool) error {
	indexModel := mongo.IndexModel{
		Options: options.Index().SetUnique(isUnique).SetName(m.name + "_" + strings.Join(fieldNames, "_") + "_index"),
	}

	keys := make(bson.D, 0, len(fieldNames))
	for _, field := range fieldNames {
		keys = append(keys, bson.E{
			Key:   field,
			Value: 1,
		})
	}
	indexModel.Keys = keys

	if _, err := m.coll.Indexes().CreateOne(ctx, indexModel); err != nil {
		return err
	}

	return nil
}

func (m *Collection) find(ctx context.Context, dest any, filter bson.M) error {
	cur, err := m.coll.Find(ctx, filter)
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return ErrNotFound
	case err != nil:
		return err
	}
	defer cur.Close(ctx)

	if err := cur.All(ctx, dest); err != nil {
		return err
	}

	if err := cur.Err(); err != nil {
		return err
	}
	return nil
}

func (m *Collection) updateOne(ctx context.Context, filter Filter, update bson.M) error {
	updateResult, err := m.coll.UpdateOne(ctx, prepareFilter(filter), update)
	switch {
	case errors.Is(err, mongo.ErrNoDocuments) || (updateResult != nil && updateResult.MatchedCount == 0):
		return ErrNotFound
	case err != nil:
		return err
	}
	return nil
}

// AsyncCollection is a wrapper for Collection with queue for asynchronous tasks.
type AsyncCollection struct {
	coll  *Collection
	queue *gorder.Gorder[string]
}

func NewAsyncCollection(ctx contem.Context, coll *Collection, workers int, lg gorder.Logger) *AsyncCollection {
	q := gorder.NewWithOptions[string](ctx, gorder.Options{
		Workers:         workers,
		Log:             lg,
		ThrowOnShutdown: true,
		Retries:         10, // TODO: add WAL
	})
	ctx.Add(q.Shutdown)

	return &AsyncCollection{
		coll:  coll,
		queue: q,
	}
}

// Insert adds a task into the queue to call Collection.Insert.
func (m *AsyncCollection) Insert(queue, name string, record any) {
	m.queue.Push(queue, name, func(ctx context.Context) error {
		return m.coll.Insert(ctx, record)
	})
}

// Replace adds a task into the queue to call Collection.Replace.
func (m *AsyncCollection) Replace(queue, name string, record any, filter Filter) {
	m.queue.Push(queue, name, func(ctx context.Context) error {
		return m.coll.Replace(ctx, record, filter)
	})
}

// SetFields adds a task into the queue to call Collection.SetFields.
func (m *AsyncCollection) SetFields(queue, name string, filter Filter, update Updates) {
	m.queue.Push(queue, name, func(ctx context.Context) error {
		return m.coll.SetFields(ctx, filter, update)
	})
}

// SetFromDiff adds a task into the queue to call Collection.SetFromDiff.
func (m *AsyncCollection) SetFromDiff(queue, name string, filter Filter, diff any) {
	m.queue.Push(queue, name, func(ctx context.Context) error {
		return m.coll.SetFromDiff(ctx, filter, diff)
	})
}

// DeleteFields adds a task into the queue to call Collection.DeleteFields.
func (m *AsyncCollection) DeleteFields(queue, name string, filter Filter, fields []string) {
	m.queue.Push(queue, name, func(ctx context.Context) error {
		return m.coll.DeleteFields(ctx, filter, fields)
	})
}

// Delete adds a task into the queue to call Collection.Delete.
func (m *AsyncCollection) Delete(queue, name string, filter Filter) {
	m.queue.Push(queue, name, func(ctx context.Context) error {
		return m.coll.Delete(ctx, filter)
	})
}

// Filter is a map containing query operators to filter documents.
type Filter map[string]any

// NewFilter creates a new Filter based on pairs.
// Pairs must be in the form NewFilter(key1, value1, key2, value2, ...)
func NewFilter(pairs ...any) Filter {
	return new(pairs...)
}

// Add adds pairs to the Filter.
func (f Filter) Add(pairs ...any) Filter {
	add(f, pairs...)
	return f
}

// Updates is a map containing fields to update.
type Updates map[string]any

// NewUpdates creates a new Updates based on pairs.
// Pairs must be in the form NewUpdates(key1, value1, key2, value2, ...)
func NewUpdates(pairs ...any) Updates {
	return new(pairs...)
}

func new(pairs ...any) map[string]any {
	out := make(map[string]any, len(pairs)/2)
	add(out, pairs...)
	return out
}

func add(m map[string]any, pairs ...any) {
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if ok && i+1 < len(pairs) {
			m[key] = pairs[i+1]
		}
	}
}

type operationDB string

const (
	in    operationDB = "$in"
	gte   operationDB = "$gte"
	set   operationDB = "$set"
	unset operationDB = "$unset"
	push  operationDB = "$push"
	pull  operationDB = "$pull"
)

func (a operationDB) String() string {
	return string(a)
}

type optionDB string

const (
	included optionDB = "included"
	from     optionDB = "from"
)

func prepareFilter(inputFilter Filter, opts ...optionDB) bson.M {
	filter := make(bson.M, len(inputFilter))
	for k, v := range inputFilter {
		for _, o := range opts {
			switch o {
			case included:
				v = bson.M{in.String(): v}
			case from:
				v = bson.M{gte.String(): v}
			}
		}
		filter[k] = v
	}
	return filter
}

func mergeFilter(base bson.M, toMerge Filter) bson.M {
	for k, v := range toMerge {
		base[k] = v
	}
	return base
}

func prepareUpdate(operation operationDB, update Updates) bson.M {
	upd := bson.D{}
	for k, v := range update {
		upd = append(upd, bson.E{Key: k, Value: v})
	}

	return bson.M{operation.String(): upd}
}

func isDuplicateErr(err error) bool {
	var e mongo.WriteException
	if errors.As(err, &e) {
		for _, we := range e.WriteErrors {
			if we.Code == 11000 {
				return true
			}
		}
	}
	return false
}

const tagName = "bson"

func diffToUpdates(diff any) (bson.M, error) {
	upd, err := processDiffStruct(diff, "")
	if err != nil {
		return nil, err
	}
	return prepareUpdate(set, upd), nil
}

func processDiffStruct(diff any, parentField string) (Updates, error) {
	req := reflect.ValueOf(diff)
	if req.Kind() != reflect.Struct {
		return nil, errm.New("only struct fields are allowed")
	}

	upd := make(Updates)
	for n := 0; n < req.NumField(); n++ {
		fieldName := req.Type().Field(n).Tag.Get(tagName)
		if parentField != "" {
			fieldName = parentField + "." + fieldName
		}

		field := req.Field(n)
		if !field.CanInterface() {
			// unexported field
			continue
		}

		kind := field.Kind()
		if kind != reflect.Ptr && kind != reflect.Array && kind != reflect.Slice && kind != reflect.Map {
			// expect pointers or slice/
			continue
		}

		if field.IsNil() {
			// nil == no update for field
			continue
		}

		if kind == reflect.Pointer {
			// get value of pointer
			field = field.Elem()
		}

		if field.Kind() == reflect.Struct {
			i := field.Interface()
			if _, ok := i.(time.Time); ok {
				upd[fieldName] = i
				continue
			}
			if _, ok := i.(time.Duration); ok {
				upd[fieldName] = i
				continue
			}

			structUpd, err := processDiffStruct(field.Interface(), fieldName)
			if err != nil {
				continue
			}
			for k, v := range structUpd {
				upd[k] = v
			}
			continue
		}

		upd[fieldName] = field.Interface()
	}

	if len(upd) == 0 {
		return nil, errm.New("updates are empty")
	}

	return upd, nil
}
