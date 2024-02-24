CREATE TABLE atx_sync_state 
(
    epoch     INT NOT NULL,
    id        CHAR(32) NOT NULL,
    requests  INT NOT NULL DEFAULT 0, 
    PRIMARY KEY (epoch, id)
) WITHOUT ROWID;