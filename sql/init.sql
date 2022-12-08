
create table if not exists sources
(
    id               serial
        constraint sources_pk
            primary key,
    url              varchar(1024),
    last_time_parsed timestamp default '0001-01-01 00:00:00 BC'::timestamp without time zone
);

create table if not exists feeds
(
    id          serial
        constraint feeds_pk
            primary key,
    title       varchar(1024),
    image_link  varchar(1024),
    source_link varchar(1024),
    category    varchar(1024),
    description text,
    datetime    timestamp
        constraint feeds_tk
            unique,
    source_id   integer,
    constraint feeds_sk_1
        unique (title, datetime)
);