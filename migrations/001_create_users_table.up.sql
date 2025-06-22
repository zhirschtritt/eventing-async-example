create table users (
    id uuid primary key default gen_random_uuid(),
    email text unique not null,
    name text not null,
    created_at timestamptz default now(),
    updated_at timestamptz default now()
);

create index idx_users_email on users(email);
create index idx_users_created_at on users(created_at); 