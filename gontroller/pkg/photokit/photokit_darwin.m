#import <AppKit/AppKit.h>
#import <AVFoundation/AVFoundation.h>
#import <Photos/Photos.h>
#include <stdlib.h>
#include <string.h>
#include "photokit.h"

// See _sb/spikes/photokit and findings "PhotoKit spike": what each request makes
// local, how long it takes, and why the waits below.

static const double TIMEOUT = 120; // seconds: a slow network, a big video

static char *dupstr(NSString *s) { return strdup(s.UTF8String ?: ""); }

static char *errstr(NSDictionary *info) {
    NSError *err = info[PHImageErrorKey];
    if (err) return dupstr(err.description);
    if ([info[PHImageCancelledKey] boolValue]) return dupstr(@"cancelled");
    return NULL;
}

int pk_status(void) {
    return (int)[PHPhotoLibrary authorizationStatusForAccessLevel:PHAccessLevelReadWrite];
}

int pk_authorize(void) {
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    __block PHAuthorizationStatus got = PHAuthorizationStatusNotDetermined;
    [PHPhotoLibrary requestAuthorizationForAccessLevel:PHAccessLevelReadWrite
                                               handler:^(PHAuthorizationStatus status) {
        got = status;
        dispatch_semaphore_signal(done);
    }];
    dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
    return (int)got;
}

static PHAsset *asset(const char *uuid) {
    // A local identifier is the asset's UUID with a suffix
    NSString *ident = [NSString stringWithFormat:@"%s/L0/001", uuid];
    return [PHAsset fetchAssetsWithLocalIdentifiers:@[ident] options:nil].firstObject;
}

// wait: for an asynchronous result. Never called on the main thread (the server's
// requests come from goroutines): the main queue is served by pk_run there.
static char *pk_wait(dispatch_semaphore_t done, char **err) {
    if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(TIMEOUT * NSEC_PER_SEC))) != 0) {
        return dupstr(@"timeout");
    }
    return *err;
}

char *pk_image(const char *uuid, int size, void **jpeg, long *len) {
    *jpeg = NULL;
    *len = 0;
    PHAsset *a = asset(uuid);
    if (!a) return dupstr(@"asset not found");
    PHImageRequestOptions *opt = [PHImageRequestOptions new];
    opt.deliveryMode = PHImageRequestOptionsDeliveryModeHighQualityFormat;
    opt.resizeMode = PHImageRequestOptionsResizeModeFast;
    opt.networkAccessAllowed = YES;
    // Synchronous: off the main thread it answers here (asynchronous results would
    // come on the main queue)
    opt.synchronous = YES;
    __block char *err = NULL;
    __block NSData *data = nil;
    [[PHImageManager defaultManager] requestImageForAsset:a
                                               targetSize:CGSizeMake(size, size)
                                              contentMode:PHImageContentModeAspectFit
                                                  options:opt
                                            resultHandler:^(NSImage *img, NSDictionary *info) {
        if ([info[PHImageResultIsDegradedKey] boolValue]) return;
        err = errstr(info);
        CGImageRef cg = img ? [img CGImageForProposedRect:NULL context:nil hints:nil] : NULL;
        if (cg) {
            NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithCGImage:cg];
            data = [rep representationUsingType:NSBitmapImageFileTypeJPEG
                                     properties:@{NSImageCompressionFactor: @0.85}];
        }
    }];
    if (!err && data.length) {
        *jpeg = malloc(data.length);
        memcpy(*jpeg, data.bytes, data.length);
        *len = (long)data.length;
    }
    return err;
}

char *pk_video(const char *uuid, int mode) {
    PHAsset *a = asset(uuid);
    if (!a) return dupstr(@"asset not found");
    PHVideoRequestOptions *opt = [PHVideoRequestOptions new];
    // Never automatic / high quality: those download the original
    opt.deliveryMode = (PHVideoRequestOptionsDeliveryMode)mode;
    opt.networkAccessAllowed = YES;
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    __block char *err = NULL;
    [[PHImageManager defaultManager] requestAVAssetForVideo:a options:opt
                                              resultHandler:^(AVAsset *av, AVAudioMix *mix, NSDictionary *info) {
        err = errstr(info);
        if (!err && !av) err = dupstr(@"no video");
        dispatch_semaphore_signal(done);
    }];
    return pk_wait(done, &err);
}

char *pk_live(const char *uuid) {
    PHAsset *a = asset(uuid);
    if (!a) return dupstr(@"asset not found");
    PHLivePhotoRequestOptions *opt = [PHLivePhotoRequestOptions new];
    opt.deliveryMode = PHImageRequestOptionsDeliveryModeHighQualityFormat;
    opt.networkAccessAllowed = YES;
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    __block char *err = NULL;
    // The still's size as the viewer's (not the maximum: that is the original)
    [[PHImageManager defaultManager] requestLivePhotoForAsset:a
                                                   targetSize:CGSizeMake(2048, 2048)
                                                  contentMode:PHImageContentModeAspectFit
                                                      options:opt
                                                resultHandler:^(PHLivePhoto *live, NSDictionary *info) {
        if ([info[PHImageResultIsDegradedKey] boolValue]) return;
        err = errstr(info);
        if (!err && !live) err = dupstr(@"no live photo");
        dispatch_semaphore_signal(done);
    }];
    return pk_wait(done, &err);
}

void pk_run(double seconds) {
    CFRunLoopRunInMode(kCFRunLoopDefaultMode, seconds, false);
}
