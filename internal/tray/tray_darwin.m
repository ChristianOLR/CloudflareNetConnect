//go:build darwin

#import <Cocoa/Cocoa.h>

extern void goOnToggleMaster(void);
extern void goOnToggleAccount(int index);
extern void goOnLinkAccount(void);
extern void goOnOpenConfig(void);
extern void goOnQuit(void);

@interface MacTrayHandler : NSObject
@property (strong, nonatomic) NSStatusItem *statusItem;
@end

static MacTrayHandler *g_trayHandler = nil;

@implementation MacTrayHandler

- (void)actionToggleMaster:(id)sender {
    goOnToggleMaster();
}

- (void)actionToggleAccount:(id)sender {
    NSMenuItem *item = (NSMenuItem *)sender;
    goOnToggleAccount((int)item.tag);
}

- (void)actionLinkAccount:(id)sender {
    goOnLinkAccount();
}

- (void)actionOpenConfig:(id)sender {
    goOnOpenConfig();
}

- (void)actionQuit:(id)sender {
    goOnQuit();
}

- (void)setupMenuWithMaster:(BOOL)masterActive accounts:(NSArray *)accounts {
    NSMenu *menu = [[NSMenu alloc] initWithTitle:@"CloudflareNetConnect"];

    // 1. Cabecera informativa con semáforo
    BOOL effectivelyActive = NO;
    if (masterActive) {
        for (NSDictionary *acc in accounts) {
            if ([acc[@"enabled"] boolValue]) {
                effectivelyActive = YES;
                break;
            }
        }
    }

    NSString *headerTitle = @"🔴 CloudflareNetConnect (Servicio Pausado)";
    if (effectivelyActive) {
        headerTitle = @"🟢 CloudflareNetConnect (Servicio Activo)";
    } else if (masterActive && accounts.count == 0) {
        headerTitle = @"🔴 CloudflareNetConnect (Sin dominios vinculados)";
    } else if (masterActive) {
        headerTitle = @"🔴 CloudflareNetConnect (Sin dominios activos)";
    }
    NSMenuItem *header = [[NSMenuItem alloc] initWithTitle:headerTitle action:nil keyEquivalent:@""];
    [header setEnabled:NO];
    [menu addItem:header];
    [menu addItem:[NSMenuItem separatorItem]];

    // 2. Interruptor Maestro
    NSString *masterTitle = masterActive ? @"⚡ Servicio de Túneles Activo (Clic para pausar)" : @"⏸ Servicio de Túneles Pausado (Clic para activar)";
    NSMenuItem *masterItem = [[NSMenuItem alloc] initWithTitle:masterTitle action:@selector(actionToggleMaster:) keyEquivalent:@""];
    [masterItem setTarget:self];
    [masterItem setState:masterActive ? NSControlStateValueOn : NSControlStateValueOff];
    [menu addItem:masterItem];
    [menu addItem:[NSMenuItem separatorItem]];

    // 3. Sección de Cuentas con bolitas semáforo verde y roja
    if (accounts.count == 0) {
        NSMenuItem *noAcc = [[NSMenuItem alloc] initWithTitle:@"(Sin cuentas vinculadas)" action:nil keyEquivalent:@""];
        [noAcc setEnabled:NO];
        [menu addItem:noAcc];
    } else {
        for (int i = 0; i < (int)accounts.count; i++) {
            NSDictionary *acc = accounts[i];
            NSString *name = acc[@"name"];
            BOOL enabled = [acc[@"enabled"] boolValue];
            BOOL isOnline = enabled && masterActive;
            NSString *dot = isOnline ? @"🟢" : @"🔴";
            NSString *title = [NSString stringWithFormat:@"%@ %@", dot, name];
            NSMenuItem *accItem = [[NSMenuItem alloc] initWithTitle:title action:@selector(actionToggleAccount:) keyEquivalent:@""];
            [accItem setTarget:self];
            [accItem setTag:i];
            [accItem setState:enabled ? NSControlStateValueOn : NSControlStateValueOff];
            [menu addItem:accItem];
        }
    }
    [menu addItem:[NSMenuItem separatorItem]];

    // 4. Acciones de Configuración
    NSMenuItem *linkItem = [[NSMenuItem alloc] initWithTitle:@"+ Vincular Cuenta..." action:@selector(actionLinkAccount:) keyEquivalent:@""];
    [linkItem setTarget:self];
    [menu addItem:linkItem];

    NSMenuItem *cfgItem = [[NSMenuItem alloc] initWithTitle:@"⚙️ Abrir Configuración..." action:@selector(actionOpenConfig:) keyEquivalent:@""];
    [cfgItem setTarget:self];
    [menu addItem:cfgItem];
    [menu addItem:[NSMenuItem separatorItem]];

    // 5. Salir
    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"❌ Salir" action:@selector(actionQuit:) keyEquivalent:@"q"];
    [quitItem setTarget:self];
    [menu addItem:quitItem];

    self.statusItem.menu = menu;
}

@end

void initMacTray(const void* iconBytes, int iconLen) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

        g_trayHandler = [[MacTrayHandler alloc] init];

        NSStatusBar *statusBar = [NSStatusBar systemStatusBar];
        g_trayHandler.statusItem = [statusBar statusItemWithLength:NSSquareStatusItemLength];

        if (iconBytes != NULL && iconLen > 0) {
            NSData *data = [NSData dataWithBytes:iconBytes length:iconLen];
            NSImage *img = [[NSImage alloc] initWithData:data];
            if (img) {
                [img setSize:NSMakeSize(18, 18)];
                [img setTemplate:NO];
                g_trayHandler.statusItem.button.image = img;
            }
        }
    }
}

void setMacMenu(int masterActive, const char* accountsJson, const void* iconBytes, int iconLen) {
    @autoreleasepool {
        NSString *jsonStr = [NSString stringWithUTF8String:accountsJson];
        NSData *jsonData = [jsonStr dataUsingEncoding:NSUTF8StringEncoding];
        NSArray *accs = @[];
        if (jsonData) {
            accs = [NSJSONSerialization JSONObjectWithData:jsonData options:0 error:nil] ?: @[];
        }

        NSData *iconData = nil;
        if (iconBytes != NULL && iconLen > 0) {
            iconData = [NSData dataWithBytes:iconBytes length:iconLen];
        }

        dispatch_async(dispatch_get_main_queue(), ^{
            if (iconData) {
                NSImage *img = [[NSImage alloc] initWithData:iconData];
                if (img) {
                    [img setSize:NSMakeSize(18, 18)];
                    [img setTemplate:NO];
                    g_trayHandler.statusItem.button.image = img;
                    g_trayHandler.statusItem.button.title = @"";
                }
            }
            [g_trayHandler setupMenuWithMaster:(masterActive != 0) accounts:accs];
        });
    }
}

void runMacLoop(void) {
    @autoreleasepool {
        [NSApp run];
    }
}

void stopMacLoop(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp stop:nil];
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSMakePoint(0, 0) modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
        [NSApp postEvent:event atStart:YES];
    });
}
