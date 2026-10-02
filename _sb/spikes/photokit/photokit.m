// A spike: the synchronous AVAsset accessors are fine here
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
#import <AppKit/AppKit.h>
#import <Photos/Photos.h>
#import <AVFoundation/AVFoundation.h>
#include <stdlib.h>
#include <string.h>
#include "photokit.h"

static char *dupstr(NSString *s) { return strdup(s.UTF8String ?: ""); }

// wait: until done or the timeout. Some PhotoKit results come on the main queue: on
// the main thread (the Go side locks main there) its run loop is turned meanwhile
static BOOL pk_wait(dispatch_semaphore_t done, double seconds) {
    NSDate *until = [NSDate dateWithTimeIntervalSinceNow:seconds];
    while ([until timeIntervalSinceNow] > 0) {
        if (dispatch_semaphore_wait(done, DISPATCH_TIME_NOW) == 0) return YES;
        if ([NSThread isMainThread]) CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.02, true);
        else usleep(20000);
    }
    return NO;
}

int pk_status(void) {
    return (int)[PHPhotoLibrary authorizationStatusForAccessLevel:PHAccessLevelReadWrite];
}

int pk_auth(void) {
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

char *pk_resources(const char *uuid) {
    PHAsset *a = asset(uuid);
    if (!a) return dupstr(@"{\"error\":\"asset not found\"}\n");
    NSMutableString *out = [NSMutableString string];
    for (PHAssetResource *r in [PHAssetResource assetResourcesForAsset:a]) {
        id local = nil, size = nil;
        @try { local = [r valueForKey:@"locallyAvailable"]; } @catch (id e) {}
        @try { size = [r valueForKey:@"fileSize"]; } @catch (id e) {}
        [out appendFormat:@"{\"type\":%ld,\"name\":\"%@\",\"size\":%@,\"local\":%@}\n",
            (long)r.type, r.originalFilename, size ?: @"null",
            local ? ([local boolValue] ? @"true" : @"false") : @"null"];
    }
    return dupstr(out);
}

pk_result pk_request(const char *uuid, int target, int network) {
    pk_result res = {0};
    res.progress = -1;
    PHAsset *a = asset(uuid);
    if (!a) {
        res.error = dupstr(@"asset not found");
        return res;
    }
    PHImageRequestOptions *opt = [PHImageRequestOptions new];
    opt.deliveryMode = PHImageRequestOptionsDeliveryModeHighQualityFormat;
    opt.resizeMode = PHImageRequestOptionsResizeModeFast;
    opt.networkAccessAllowed = network != 0;
    // Synchronous: asynchronous results are delivered on the main queue, which a
    // command-line tool does not run — the request would never come back
    opt.synchronous = YES;
    __block double progress = -1;
    opt.progressHandler = ^(double p, NSError *err, BOOL *stop, NSDictionary *info) { progress = p; };

    __block pk_result r = res;
    NSDate *start = [NSDate date];
    [[PHImageManager defaultManager] requestImageForAsset:a
                                               targetSize:CGSizeMake(target, target)
                                              contentMode:PHImageContentModeAspectFit
                                                  options:opt
                                            resultHandler:^(NSImage *img, NSDictionary *info) {
        if ([info[PHImageResultIsDegradedKey] boolValue]) {
            r.degraded++;
            return; // the final one follows
        }
        r.seconds = -[start timeIntervalSinceNow];
        r.inCloud = [info[PHImageResultIsInCloudKey] boolValue];
        NSError *err = info[PHImageErrorKey];
        if (err) r.error = dupstr(err.description);
        if ([info[PHImageCancelledKey] boolValue] && !r.error) r.error = dupstr(@"cancelled");
        if (img) {
            // The bitmap's own pixels: NSImage sizes are points (×2 on Retina)
            CGImageRef cg = [img CGImageForProposedRect:NULL context:nil hints:nil];
            r.width = (int)CGImageGetWidth(cg);
            r.height = (int)CGImageGetHeight(cg);
        }
    }];
    if (r.seconds == 0) r.seconds = -[start timeIntervalSinceNow];
    r.progress = progress;
    return r;
}

pk_video_result pk_video(const char *uuid, int network) {
    pk_video_result res = {0};
    PHAsset *a = asset(uuid);
    if (!a) {
        res.error = dupstr(@"asset not found");
        return res;
    }
    PHVideoRequestOptions *opt = [PHVideoRequestOptions new];
    opt.deliveryMode = PHVideoRequestOptionsDeliveryModeMediumQualityFormat;
    opt.networkAccessAllowed = network != 0;
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    __block pk_video_result r = res;
    NSDate *start = [NSDate date];
    [[PHImageManager defaultManager] requestAVAssetForVideo:a options:opt
                                              resultHandler:^(AVAsset *av, AVAudioMix *mix, NSDictionary *info) {
        r.seconds = -[start timeIntervalSinceNow];
        r.inCloud = [info[PHImageResultIsInCloudKey] boolValue];
        NSError *err = info[PHImageErrorKey];
        if (err) r.error = dupstr(err.description);
        if ([av isKindOfClass:[AVURLAsset class]]) r.url = dupstr(((AVURLAsset *)av).URL.absoluteString);
        else if (av) r.url = dupstr(NSStringFromClass([av class]));
        AVAssetTrack *t = [av tracksWithMediaType:AVMediaTypeVideo].firstObject;
        if (t) {
            r.width = (int)t.naturalSize.width;
            r.height = (int)t.naturalSize.height;
        }
        if (av) r.duration = CMTimeGetSeconds(av.duration);
        dispatch_semaphore_signal(done);
    }];
    if (!pk_wait(done, 180)) {
        r.seconds = -[start timeIntervalSinceNow];
        r.error = dupstr(@"timeout");
    }
    return r;
}

pk_result pk_live(const char *uuid, int target, int network) {
    pk_result res = {0};
    res.progress = -1;
    PHAsset *a = asset(uuid);
    if (!a) {
        res.error = dupstr(@"asset not found");
        return res;
    }
    PHLivePhotoRequestOptions *opt = [PHLivePhotoRequestOptions new];
    opt.deliveryMode = PHImageRequestOptionsDeliveryModeHighQualityFormat;
    opt.networkAccessAllowed = network != 0;
    __block double progress = -1;
    opt.progressHandler = ^(double p, NSError *err, BOOL *stop, NSDictionary *info) { progress = p; };
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    __block pk_result r = res;
    NSDate *start = [NSDate date];
    [[PHImageManager defaultManager] requestLivePhotoForAsset:a
                                                   targetSize:CGSizeMake(target, target)
                                                  contentMode:PHImageContentModeAspectFit
                                                      options:opt
                                                resultHandler:^(PHLivePhoto *live, NSDictionary *info) {
        if ([info[PHImageResultIsDegradedKey] boolValue]) {
            r.degraded++;
            return;
        }
        r.seconds = -[start timeIntervalSinceNow];
        r.inCloud = [info[PHImageResultIsInCloudKey] boolValue];
        NSError *err = info[PHImageErrorKey];
        if (err) r.error = dupstr(err.description);
        if (live) {
            r.width = (int)live.size.width;
            r.height = (int)live.size.height;
        }
        dispatch_semaphore_signal(done);
    }];
    if (!pk_wait(done, 180)) {
        r.seconds = -[start timeIntervalSinceNow];
        r.error = dupstr(@"timeout");
    }
    r.progress = progress;
    return r;
}
