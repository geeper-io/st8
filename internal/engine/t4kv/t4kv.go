package t4kv

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/t4db/t4"
	"github.com/t4db/t4/pkg/object"

	"github.com/geeper-io/st8/internal/engine"
	"github.com/geeper-io/st8/internal/model"
)

// Key layout:
//
//	/ns/{ns}/meta                        → JSON NamespaceMeta
//	/ns/{ns}/b/{branch}/meta             → JSON BranchMeta
//	/ns/{ns}/b/{branch}/obj/{key}        → raw object value
//	/ns/{ns}/b/{branch}/r/{revID_hex16}  → JSON Revision (delta only)
//	/ns/{ns}/cp/{name}                   → JSON Checkpoint (full snapshot)

const (
	keyNsMeta     = "/ns/%s/meta"
	keyBranchMeta = "/ns/%s/b/%s/meta"
	keyObjPrefix  = "/ns/%s/b/%s/obj/"
	keyObj        = "/ns/%s/b/%s/obj/%s"
	keyRevPrefix  = "/ns/%s/b/%s/r/"
	keyRev        = "/ns/%s/b/%s/r/%016x"
	keyCpPrefix   = "/ns/%s/cp/"
	keyCp         = "/ns/%s/cp/%s"
	keyNsPrefix   = "/ns/"
)

type Config struct {
	Logger            *logrus.Logger
	MetricsRegisterer prometheus.Registerer

	// S3Bucket, when non-empty, enables WAL archiving and checkpointing to S3.
	// AWS credentials are resolved via the standard credential chain
	// (env vars, ~/.aws/credentials, EC2/ECS metadata, etc.) unless
	// S3AccessKeyID and S3SecretAccessKey are set.
	S3Bucket string
	// S3Prefix is an optional key prefix inside the bucket (may be empty).
	S3Prefix string
	// S3Endpoint overrides the default AWS endpoint, e.g. for MinIO or
	// other S3-compatible object stores (http://minio:9000).
	S3Endpoint string
	// S3Region overrides the AWS region. When empty the region is resolved
	// from the standard AWS chain.
	S3Region string
	// S3Profile selects a named profile from ~/.aws/config. Ignored when
	// S3AccessKeyID is set.
	S3Profile string
	// S3AccessKeyID and S3SecretAccessKey provide static credentials,
	// bypassing the default credential chain. Both must be set together.
	S3AccessKeyID     string
	S3SecretAccessKey string
}

type Engine struct {
	node *t4.Node
}

func New(ctx context.Context, stateDir string, cfg Config) (*Engine, error) {
	dataDir := filepath.Join(stateDir, "engine")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	t4cfg := t4.Config{
		DataDir:           dataDir,
		Logger:            cfg.Logger,
		MetricsRegisterer: cfg.MetricsRegisterer,
	}
	if cfg.S3Bucket != "" {
		store, err := object.NewS3StoreFromConfig(ctx, object.S3Config{
			Bucket:          cfg.S3Bucket,
			Prefix:          cfg.S3Prefix,
			Endpoint:        cfg.S3Endpoint,
			Region:          cfg.S3Region,
			Profile:         cfg.S3Profile,
			AccessKeyID:     cfg.S3AccessKeyID,
			SecretAccessKey: cfg.S3SecretAccessKey,
		})
		if err != nil {
			return nil, fmt.Errorf("init S3 store: %w", err)
		}
		t4cfg.ObjectStore = store
	}
	node, err := t4.Open(t4cfg)
	if err != nil {
		return nil, err
	}
	return &Engine{node: node}, nil
}

func (e *Engine) Close() error { return e.node.Close() }

func (e *Engine) Ping(_ context.Context) error {
	_, err := e.node.Get(keyNsPrefix)
	return err
}

// GetObjects returns all current objects for the given namespace/branch.
func (e *Engine) GetObjects(_ context.Context, ns, branch string) (map[string]string, error) {
	prefix := fmt.Sprintf(keyObjPrefix, ns, branch)
	kvs, err := e.node.List(prefix)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		key := strings.TrimPrefix(kv.Key, prefix)
		out[key] = string(kv.Value)
	}
	return out, nil
}

// Commit atomically writes object changes, a revision record, and the branch
// head in a single t4 transaction. Returns the assigned revision ID.
func (e *Engine) Commit(_ context.Context, req engine.CommitRequest) (int64, error) {
	// Read namespace meta to get/increment the revision counter.
	meta, err := e.getNsMeta(req.Namespace)
	if err != nil {
		return 0, err
	}
	revID := meta.NextRevision
	meta.NextRevision++
	if meta.ActiveBranch == "" {
		meta.ActiveBranch = req.Branch
	}

	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	rev := model.Revision{
		ID:        revID,
		ParentID:  req.ParentRevID,
		Namespace: req.Namespace,
		Branch:    req.Branch,
		Message:   req.Message,
		CreatedAt: createdAt,
		Changes:   req.Changes,
	}

	branchMeta := model.BranchMeta{
		Name:         req.Branch,
		HeadRevision: revID,
		BaseRevision: req.BaseRevision,
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return 0, err
	}
	revJSON, err := json.Marshal(rev)
	if err != nil {
		return 0, err
	}
	branchJSON, err := json.Marshal(branchMeta)
	if err != nil {
		return 0, err
	}

	ops := make([]t4.TxnOp, 0, 3+len(req.Puts)+len(req.Deletes))
	ops = append(ops,
		t4.TxnOp{Type: t4.TxnPut, Key: fmt.Sprintf(keyNsMeta, req.Namespace), Value: metaJSON},
		t4.TxnOp{Type: t4.TxnPut, Key: fmt.Sprintf(keyBranchMeta, req.Namespace, req.Branch), Value: branchJSON},
		t4.TxnOp{Type: t4.TxnPut, Key: fmt.Sprintf(keyRev, req.Namespace, req.Branch, revID), Value: revJSON},
	)
	for k, v := range req.Puts {
		ops = append(ops, t4.TxnOp{Type: t4.TxnPut, Key: fmt.Sprintf(keyObj, req.Namespace, req.Branch, k), Value: []byte(v)})
	}
	for _, k := range req.Deletes {
		ops = append(ops, t4.TxnOp{Type: t4.TxnDelete, Key: fmt.Sprintf(keyObj, req.Namespace, req.Branch, k)})
	}

	ctx := context.Background()
	_, err = e.node.Txn(ctx, t4.TxnRequest{Success: ops})
	if err != nil {
		return 0, err
	}
	return revID, nil
}

// GetRevision returns the revision record for a specific revision ID.
func (e *Engine) GetRevision(_ context.Context, ns, branch string, revID int64) (*model.Revision, error) {
	kv, err := e.node.Get(fmt.Sprintf(keyRev, ns, branch, revID))
	if err != nil {
		return nil, err
	}
	if kv == nil {
		return nil, nil
	}
	var rev model.Revision
	if err := json.Unmarshal(kv.Value, &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}

// DeleteRevision removes a revision record.
func (e *Engine) DeleteRevision(ctx context.Context, ns, branch string, revID int64) error {
	_, err := e.node.Delete(ctx, fmt.Sprintf(keyRev, ns, branch, revID))
	return err
}

// GetNamespaceMeta returns namespace metadata, creating a zero-value if absent.
func (e *Engine) GetNamespaceMeta(_ context.Context, ns string) (*model.NamespaceMeta, error) {
	m, err := e.getNsMeta(ns)
	return m, err
}

func (e *Engine) getNsMeta(ns string) (*model.NamespaceMeta, error) {
	kv, err := e.node.Get(fmt.Sprintf(keyNsMeta, ns))
	if err != nil {
		return nil, err
	}
	if kv == nil || len(kv.Value) == 0 {
		return &model.NamespaceMeta{NextRevision: 1}, nil
	}
	var m model.NamespaceMeta
	if err := json.Unmarshal(kv.Value, &m); err != nil {
		return nil, err
	}
	if m.NextRevision == 0 {
		m.NextRevision = 1
	}
	return &m, nil
}

// SaveNamespaceMeta persists namespace metadata.
func (e *Engine) SaveNamespaceMeta(ctx context.Context, ns string, m *model.NamespaceMeta) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = e.node.Put(ctx, fmt.Sprintf(keyNsMeta, ns), data, 0)
	return err
}

// GetBranchMeta returns branch metadata, or nil if the branch does not exist.
func (e *Engine) GetBranchMeta(_ context.Context, ns, branch string) (*model.BranchMeta, error) {
	kv, err := e.node.Get(fmt.Sprintf(keyBranchMeta, ns, branch))
	if err != nil {
		return nil, err
	}
	if kv == nil || len(kv.Value) == 0 {
		return nil, nil
	}
	var m model.BranchMeta
	if err := json.Unmarshal(kv.Value, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// SaveBranchMeta persists branch metadata.
func (e *Engine) SaveBranchMeta(ctx context.Context, ns, branch string, m *model.BranchMeta) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = e.node.Put(ctx, fmt.Sprintf(keyBranchMeta, ns, branch), data, 0)
	return err
}

// GetCheckpoint returns a checkpoint by name, or nil if not found.
func (e *Engine) GetCheckpoint(_ context.Context, ns, name string) (*model.Checkpoint, error) {
	kv, err := e.node.Get(fmt.Sprintf(keyCp, ns, name))
	if err != nil {
		return nil, err
	}
	if kv == nil || len(kv.Value) == 0 {
		return nil, nil
	}
	var cp model.Checkpoint
	if err := json.Unmarshal(kv.Value, &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

// SaveCheckpoint persists a checkpoint record.
func (e *Engine) SaveCheckpoint(ctx context.Context, cp *model.Checkpoint) error {
	data, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	_, err = e.node.Put(ctx, fmt.Sprintf(keyCp, cp.Namespace, cp.Name), data, 0)
	return err
}

// DeleteCheckpoint removes a checkpoint.
func (e *Engine) DeleteCheckpoint(ctx context.Context, ns, name string) error {
	_, err := e.node.Delete(ctx, fmt.Sprintf(keyCp, ns, name))
	return err
}

// ListCheckpoints returns all checkpoint names for a namespace.
func (e *Engine) ListCheckpoints(_ context.Context, ns string) ([]string, error) {
	prefix := fmt.Sprintf(keyCpPrefix, ns)
	kvs, err := e.node.List(prefix)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(kvs))
	for _, kv := range kvs {
		names = append(names, strings.TrimPrefix(kv.Key, prefix))
	}
	return names, nil
}

// ListBranches returns all branch names for a namespace.
func (e *Engine) ListBranches(_ context.Context, ns string) ([]string, error) {
	// Branch meta keys: /ns/{ns}/b/{branch}/meta
	// List /ns/{ns}/b/ and extract the branch segment.
	prefix := fmt.Sprintf("/ns/%s/b/", ns)
	kvs, err := e.node.List(prefix)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, kv := range kvs {
		rel := strings.TrimPrefix(kv.Key, prefix)
		branch := strings.SplitN(rel, "/", 2)[0]
		seen[branch] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names, nil
}

// ListNamespaces returns all namespace names known to the engine.
func (e *Engine) ListNamespaces(_ context.Context) ([]string, error) {
	// Namespace meta keys: /ns/{ns}/meta — list /ns/ and collect top-level segments.
	kvs, err := e.node.List(keyNsPrefix)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, kv := range kvs {
		rel := strings.TrimPrefix(kv.Key, keyNsPrefix)
		ns := strings.SplitN(rel, "/", 2)[0]
		seen[ns] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names, nil
}
