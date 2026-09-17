package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// RelayChain holds the schema definition for the RelayChain entity.
type RelayChain struct {
	ent.Schema
}

// Fields of the RelayChain.
func (RelayChain) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Positive().
			Comment("Relay chain id | Relay 返回的链 ID"),
		field.String("slug").
			Optional().
			Comment("Relay original chain slug | Relay 原始链标识"),
		field.String("name").
			NotEmpty().
			Comment("Chain display name | 链展示名称"),
		field.String("logo_url").
			Optional().
			Comment("Chain logo URL | 链 logo 地址"),
		field.String("type").
			Default("evm").
			Comment("Chain type | 链类型"),
		field.String("vm_type").
			Optional().
			Comment("Relay vmType | Relay 返回的 vmType"),
		field.String("protocol").
			Optional().
			Comment("Relay protocol | Relay 返回的 protocol"),
		field.String("base_chain_id").
			Optional().
			Comment("Relay baseChainId | Relay 返回的 baseChainId"),
		field.String("explorer_url").
			Optional().
			Comment("Explorer URL | 区块浏览器地址"),
		field.String("explorer_name").
			Optional().
			Comment("Explorer name | 区块浏览器名称"),
		field.String("rpc_url").
			Optional().
			Comment("HTTP RPC URL | HTTP RPC 地址"),
		field.String("ws_rpc_url").
			Optional().
			Comment("WebSocket RPC URL | WebSocket RPC 地址"),
		field.String("native_symbol").
			Optional().
			Comment("Native token symbol | 原生币符号"),
		field.Int("native_decimals").
			Default(0).
			Comment("Native token decimals | 原生币精度"),
		field.Bool("deposit_enabled").
			Default(false).
			Comment("Whether Relay deposit is enabled | Relay 是否允许该链充值"),
		field.String("token_support").
			Optional().
			Comment("Relay token support strategy | Relay 返回的 token 支持策略"),
		field.Bool("disabled").
			Default(false).
			Comment("Whether Relay disabled this chain | Relay 是否已禁用该链"),
		field.Bool("supported").
			Default(true).
			Comment("Whether Relay still supports this chain | Relay 当前是否还支持这条链"),
		field.Bool("enabled").
			Default(true).
			Comment("Whether platform enables this chain | 平台是否启用这条链"),
		field.Text("raw_data").
			Optional().
			Comment("Raw Relay chain payload | Relay 原始返回数据"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("Create time | 创建时间"),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("Update time | 更新时间"),
	}
}

// Edges of the RelayChain.
func (RelayChain) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tokens", RelayToken.Type),
	}
}

func (RelayChain) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.WithComments(true),
		schema.Comment("Relay chain table | Relay 链信息表"),
		entsql.Annotation{Table: "relay_chain"},
	}
}
