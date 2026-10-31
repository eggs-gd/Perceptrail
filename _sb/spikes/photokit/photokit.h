// PhotoKit, as plain C for cgo. Every call blocks until Photos answers.

// pk_status: the authorization status without asking (PHAuthorizationStatus)
int pk_status(void);
// pk_auth: asks for read access (the system prompt the first time); the status after
int pk_auth(void);

// pk_resources: the asset's resources as JSON lines — type, file name, size, and
// whether the file is on this Mac (private KVC: fine for a spike, not for the product)
char *pk_resources(const char *uuid);

typedef struct {
    double seconds;     // until the final (not degraded) image or an error
    int width, height;  // of the image handed over, pixels
    int inCloud;        // Photos says the image needs the network
    int degraded;       // how many degraded images came first
    double progress;    // the last download progress reported (0..1), -1 = none
    char *error;        // NULL, or the error (free it)
} pk_result;

// pk_request: an image of at most target×target pixels, as the viewer would ask;
// network: may Photos download it
pk_result pk_request(const char *uuid, int target, int network);

typedef struct {
    double seconds;
    int width, height;  // the video track's size
    double duration;    // seconds
    int inCloud;
    char *url;          // where AVFoundation reads it from (a file in the library?), or NULL
    char codec[5];      // the video track's FourCC (avc1, hvc1, …)
    char *error;
} pk_video_result;

// pk_video: the asset's video as a player would ask; mode: PHVideoRequestOptionsDeliveryMode
// (0 automatic, 1 high quality, 2 medium quality, 3 fast)
pk_video_result pk_video(const char *uuid, int network, int mode);

// pk_live: a Live Photo of at most target×target, as the viewer would ask; the
// result's width/height are the still's (an unknown size: 0)
pk_result pk_live(const char *uuid, int target, int network);
