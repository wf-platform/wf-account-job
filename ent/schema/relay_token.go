package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RelayToken holds the schema definition for the RelayToken entity.
type RelayToken struct {
	ent.Schema
}

// Fields of the RelayToken.
func (RelayToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("token_id").
			NotEmpty().
			Comment("Relay token id | Relay 返回的 Token 唯一标识"),
		field.Int64("chain_id").
			Positive().
			Comment("Relay chain id | 所属链 ID"),
		field.String("address").
			NotEmpty().
			Comment("Token contract address | Token 合约地址"),
		field.String("name").
			Optional().
			Comment("Token name | Token 名称"),
		field.String("symbol").
			NotEmpty().
			Comment("Token symbol | Token 符号"),
		field.String("logo_url").
			Optional().
			Comment("Token logo URL | Token logo 地址"),
		field.Int("decimals").
			NonNegative().
			Comment("Token decimals | Token 精度"),
		field.Bool("native").
			Default(false).
			Comment("Whether token is native | 是否为链原生币"),
		field.Bool("stablecoin").
			Default(false).
			Comment("Whether token is a stablecoin | 是否稳定币"),
		field.Bool("supports_bridging").
			Default(false).
			Comment("Whether Relay supports bridging | Relay 是否支持桥接"),
		field.Bool("supports_permit").
			Default(false).
			Comment("Whether Relay supports permit | Relay 是否支持 permit"),
		field.Bool("is_featured").
			Default(false).
			Comment("Whether token appears in featuredTokens | 是否出现在 featuredTokens"),
		field.Bool("is_solver").
			Default(false).
			Comment("Whether token appears in solverCurrencies | 是否出现在 solverCurrencies"),
		field.Bool("is_erc20").
			Default(false).
			Comment("Whether token appears in erc20Currencies | 是否出现在 erc20Currencies"),
		field.Bool("supported").
			Default(true).
			Comment("Whether Relay still supports this token | Relay 当前是否还支持这个币"),
		field.Text("raw_data").
			Optional().
			Comment("Normalized Relay token snapshot | Relay 归一化后的 Token 快照"),
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

// Edges of the RelayToken.
func (RelayToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("chain", RelayChain.Type).
			Ref("tokens").
			Field("chain_id").
			Required().
			Unique(),
	}
}

func (RelayToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token_id", "chain_id").Unique(),
		index.Fields("chain_id", "address").Unique(),
	}
}

func (RelayToken) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.WithComments(true),
		schema.Comment("Relay token table | Relay Token 信息表"),
		entsql.Annotation{Table: "relay_token"},
	}
}
