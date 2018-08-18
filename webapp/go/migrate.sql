ALTER TABLE candidates ADD voted_count INTEGER default 0;
ALTER TABLE users ADD voted_count INTEGER default 0;
ALTER TABLE votes ADD voted_count INTEGER default 0;
ALTER TABLE votes change keyword keyword varchar(191);

ALTER TABLE votes ADD INDEX candidate_id_voted_count_idx(candidate_id,voted_count DESC);
ALTER TABLE votes ADD INDEX keyword_idx(keyword);
