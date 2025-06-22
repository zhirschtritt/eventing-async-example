create table events (
    id uuid primary key default gen_random_uuid(),
    sequence bigserial unique not null,
    type text not null,
    aggregate_type text not null,
    aggregate_id text not null,
    version integer not null,
    timestamp timestamptz not null,
    data jsonb not null,
    created_at timestamptz default now()
);

create index idx_events_type on events(type);
create index idx_events_timestamp on events(timestamp);
create index idx_events_aggregate_type_id_version on events(aggregate_type, aggregate_id, version);
create index idx_events_sequence on events(sequence);