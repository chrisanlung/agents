"use client";

import { useActionState, useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { loginAction, type LoginFormState } from "./actions";

const schema = z.object({
  identifier: z
    .string()
    .min(3, "Masukkan email atau username yang valid")
    .max(320, "Masukkan email atau username yang valid")
    .refine(
      (v) =>
        /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v) ||
        /^[a-zA-Z0-9._]{3,50}$/.test(v),
      "Masukkan email atau username yang valid"
    ),
  password: z.string().min(1, "Kata sandi wajib diisi"),
});

type FormValues = z.infer<typeof schema>;

const initialState: LoginFormState = { status: "idle" };

export function LoginForm() {
  const [state, formAction, isPending] = useActionState(
    loginAction,
    initialState
  );

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { identifier: "", password: "" },
  });

  useEffect(() => {
    // state can be undefined briefly when the server action calls redirect():
    // useActionState returns nothing for that round-trip, so guard with ?.
    if (state?.status === "error" && state.toast) {
      toast.error(state.toast);
    }
  }, [state]);

  useEffect(() => {
    if (state?.status === "error" && state.fieldErrors) {
      for (const [field, messages] of Object.entries(state.fieldErrors)) {
        form.setError(field as keyof FormValues, {
          message: messages[0],
        });
      }
    }
  }, [state, form]);

  return (
    <Form {...form}>
      <form action={formAction} className="space-y-6">
        <FormField
          control={form.control}
          name="identifier"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Email atau Username</FormLabel>
              <FormControl>
                <Input
                  type="text"
                  autoComplete="username"
                  placeholder="alice@spa.com atau alice"
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="password"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Kata Sandi</FormLabel>
              <FormControl>
                <Input
                  type="password"
                  autoComplete="current-password"
                  placeholder="••••••••••"
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <Button
          type="submit"
          className="w-full"
          disabled={isPending}
          aria-busy={isPending}
        >
          {isPending ? (
            <>
              <Loader2 className="animate-spin" aria-hidden="true" />
              Sedang masuk…
            </>
          ) : (
            "Masuk"
          )}
        </Button>
      </form>
    </Form>
  );
}
