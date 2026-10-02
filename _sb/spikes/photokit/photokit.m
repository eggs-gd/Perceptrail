#import <AppKit/AppKit.h>
#import <Photos/Photos.h>
#include <stdlib.h>
#include <string.h>
#include "photokit.h"

static char *dupstr(NSString *s) { return strdup(s.UTF8String ?: ""); }

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
    opt.synchronous = NO;
    __block double progress = -1;
    opt.progressHandler = ^(double p, NSError *err, BOOL *stop, NSDictionary *info) { progress = p; };

    dispatch_semaphore_t done = dispatch_semaphore_create(0);
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
            NSImageRep *rep = img.representations.firstObject;
            r.width = (int)rep.pixelsWide;
            r.height = (int)rep.pixelsHigh;
        }
        dispatch_semaphore_signal(done);
    }];
    // Network requests may take a while; a spike gives up after two minutes
    if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 120 * NSEC_PER_SEC)) != 0) {
        r.seconds = -[start timeIntervalSinceNow];
        r.error = dupstr(@"timeout");
    }
    r.progress = progress;
    return r;
}
