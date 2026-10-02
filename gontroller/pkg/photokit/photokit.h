// PhotoKit, as plain C for cgo (darwin). Every request blocks until Photos answers;
// it returns NULL on success or an error (free it).

// pk_status / pk_authorize: PHAuthorizationStatus for read access; authorize asks
// the first time (the prompt names the app that started the process)
int pk_status(void);
int pk_authorize(void);

// pk_image: an image of at most size×size — Photos makes that rendition local in its
// library (recipe 65741, ~2048 px) when it is only in iCloud
char *pk_image(const char *uuid, int size);

// pk_video: the video in a delivery mode (2 medium: 720p, HEVC for iPhone videos;
// 3 fast: H.264 360p) — Photos makes that rendition local
char *pk_video(const char *uuid, int mode);

// pk_live: a Live Photo — Photos makes its motion local (H.264)
char *pk_live(const char *uuid);

// pk_run: turns the main run loop for up to seconds (on the main thread only): some
// PhotoKit results are delivered on the main queue
void pk_run(double seconds);
